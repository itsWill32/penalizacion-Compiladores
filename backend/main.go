package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Email     string             `json:"email" bson:"email"`
	Code      string             `json:"code" bson:"code"`
	Name      string             `json:"name" bson:"name"`
	LastName  string             `json:"last_name" bson:"last_name"`
	ImageURL  string             `json:"image_url" bson:"image_url"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
}

type RegisterRequest struct {
	Email string `json:"email"`
}

type LoginRequest struct {
	Code string `json:"code"`
}

type ResendEmail struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

type Database struct {
	client   *mongo.Client
	database *mongo.Database
	users    *mongo.Collection
}

var database *Database

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No se encontró archivo .env, usando variables de entorno del sistema")
	} else {
		log.Println("✅ Archivo .env cargado correctamente")
	}

	resendKey := os.Getenv("RESEND_API_KEY")
	mongoURI := os.Getenv("MONGODB_URI")

	if resendKey == "" {
		log.Println("⚠️  RESEND_API_KEY no configurada - emails se mostrarán en consola")
	} else {
		log.Println("✅ RESEND_API_KEY configurada correctamente")
	}

	if mongoURI == "" {
		log.Fatal("❌ MONGODB_URI es requerida")
	}

	db, err := connectMongoDB()
	if err != nil {
		log.Fatal("Error conectando a MongoDB Atlas:", err)
	}
	defer db.client.Disconnect(context.TODO())

	database = db

	if err := createIndexes(); err != nil {
		log.Fatal("Error creando índices:", err)
	}

	os.MkdirAll("uploads", 0755)

	r := mux.NewRouter()

	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/register", handleRegister).Methods("POST")
	api.HandleFunc("/login", handleLogin).Methods("POST")
	api.HandleFunc("/user/{code}", handleGetUser).Methods("GET")
	api.HandleFunc("/user/{code}", handleUpdateUser).Methods("PUT")

	r.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads/"))))

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173", "http://localhost:3000"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	handler := c.Handler(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 Servidor iniciado en puerto %s\n", port)
	fmt.Println("📧 Email provider: Resend")
	fmt.Println("🗄️  Base de datos: MongoDB Atlas")
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func connectMongoDB() (*Database, error) {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		return nil, fmt.Errorf("MONGODB_URI no está configurada")
	}

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	db := client.Database("userapp")
	users := db.Collection("users")

	fmt.Println("✅ Conectado exitosamente a MongoDB Atlas")

	return &Database{
		client:   client,
		database: db,
		users:    users,
	}, nil
}

func createIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	emailIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	codeIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "code", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	_, err := database.users.Indexes().CreateMany(ctx, []mongo.IndexModel{emailIndex, codeIndex})
	if err != nil {
		return err
	}

	fmt.Println("✅ Índices creados en MongoDB")
	return nil
}

func generateCode() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := database.users.CountDocuments(ctx, bson.D{})
	if err != nil {
		return "", err
	}

	nextID := int(count) + 1
	code := fmt.Sprintf("A%02d-%d", nextID, nextID)
	return code, nil
}

func sendEmail(toEmail, code string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
		fmt.Printf("📧 EMAIL SIMULADO (RESEND_API_KEY no configurada)\n")
		fmt.Printf(strings.Repeat("=", 60) + "\n")
		fmt.Printf("Para: %s\n", toEmail)
		fmt.Printf("Asunto: Tu código de acceso - UserApp\n")
		fmt.Printf(strings.Repeat("-", 60) + "\n")
		fmt.Printf("🔑 CÓDIGO DE ACCESO: %s\n", code)
		fmt.Printf(strings.Repeat("=", 60) + "\n\n")
		return nil
	}

	email := ResendEmail{
		From:    "UserApp <onboarding@resend.dev>",
		To:      []string{toEmail},
		Subject: "Tu código de acceso - UserApp",
		HTML: fmt.Sprintf(`
			<!DOCTYPE html>
			<html>
			<head>
				<meta charset="UTF-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0">
				<title>Código de Acceso</title>
			</head>
			<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
						max-width: 600px; margin: 0 auto; padding: 20px; background-color: #f8f9fa;">
				
				<div style="background: white; border-radius: 12px; padding: 40px; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
					<!-- Header -->
					<div style="text-align: center; margin-bottom: 30px;">
						<h1 style="color: #667eea; margin: 0; font-size: 28px; font-weight: 600;">
							UserApp
						</h1>
						<p style="color: #6c757d; margin: 5px 0 0 0; font-size: 14px;">
							Sistema de Registro
						</p>
					</div>
					
					<!-- Main Content -->
					<div style="text-align: center;">
						<h2 style="color: #333; margin-bottom: 20px; font-size: 24px;">
							¡Bienvenido! 🎉
						</h2>
						
						<p style="color: #555; font-size: 16px; line-height: 1.5; margin-bottom: 30px;">
							Hemos recibido tu solicitud de registro. Aquí tienes tu código de acceso único:
						</p>
						
						<!-- Code Box -->
						<div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
								   color: white;
								   padding: 30px;
								   border-radius: 12px;
								   margin: 30px 0;
								   box-shadow: 0 8px 25px rgba(102, 126, 234, 0.3);
								   border: 2px solid rgba(255,255,255,0.1);">
							<div style="font-size: 14px; opacity: 0.9; margin-bottom: 10px; text-transform: uppercase; letter-spacing: 1px;">
								Tu Código de Acceso
							</div>
							<div style="font-size: 36px; font-weight: 700; letter-spacing: 3px; margin: 0;">
								%s
							</div>
						</div>
						
						<!-- Instructions -->
						<div style="background: #e3f2fd; border-left: 4px solid #2196f3; padding: 20px; border-radius: 8px; margin: 25px 0;">
							<p style="margin: 0; color: #1976d2; font-size: 14px; text-align: left;">
								<strong>📌 Instrucciones:</strong><br>
								1. Copia exactamente este código<br>
								2. Ve a la página de inicio de sesión<br>
								3. Pega el código en el campo correspondiente<br>
								4. ¡Listo! Ya puedes acceder a tu perfil
							</p>
						</div>
						
						<p style="color: #666; font-size: 14px; margin-top: 30px;">
							Este código es único y válido solo para tu cuenta.<br>
							No lo compartas con nadie más.
						</p>
					</div>
					
					<!-- Footer -->
					<div style="margin-top: 40px; padding-top: 20px; border-top: 1px solid #eee; text-align: center;">
						<p style="color: #999; font-size: 12px; margin: 0;">
							Este es un mensaje automático, por favor no respondas a este correo.
						</p>
						<p style="color: #999; font-size: 12px; margin: 5px 0 0 0;">
							© 2024 UserApp - Sistema de Registro con Códigos Únicos
						</p>
					</div>
				</div>
			</body>
			</html>
		`, code),
	}

	jsonData, err := json.Marshal(email)
	if err != nil {
		return fmt.Errorf("error creando JSON: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creando petición: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error enviando petición: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error de Resend API: status %d, response: %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ Email enviado exitosamente a %s", toEmail)
	return nil
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "Email requerido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var existingUser User
	err := database.users.FindOne(ctx, bson.M{"email": req.Email}).Decode(&existingUser)
	if err == nil {
		http.Error(w, "El email ya está registrado", http.StatusBadRequest)
		return
	}
	if err != mongo.ErrNoDocuments {
		log.Printf("Error verificando email: %v", err)
		http.Error(w, "Error de base de datos", http.StatusInternalServerError)
		return
	}

	code, err := generateCode()
	if err != nil {
		log.Printf("Error generando código: %v", err)
		http.Error(w, "Error generando código", http.StatusInternalServerError)
		return
	}

	user := User{
		Email:     req.Email,
		Code:      code,
		Name:      "",
		LastName:  "",
		ImageURL:  "",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result, err := database.users.InsertOne(ctx, user)
	if err != nil {
		log.Printf("Error insertando usuario: %v", err)
		http.Error(w, "Error guardando usuario", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Usuario creado con ID: %v", result.InsertedID)

	if err := sendEmail(req.Email, code); err != nil {
		log.Printf("❌ Error enviando email: %v", err)
	} else {
		log.Printf("✅ Código %s enviado a %s", code, req.Email)
	}

	response := map[string]string{
		"message": "Usuario registrado correctamente. Revisa tu email para obtener el código de acceso.",
	}

	if os.Getenv("RESEND_API_KEY") == "" {
		response["dev_code"] = code
		response["dev_note"] = "RESEND_API_KEY no configurada - código mostrado solo para desarrollo"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		http.Error(w, "Código requerido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var user User
	err := database.users.FindOne(ctx, bson.M{"code": req.Code}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		http.Error(w, "Código inválido", http.StatusUnauthorized)
		return
	}
	if err != nil {
		log.Printf("Error buscando usuario: %v", err)
		http.Error(w, "Error de base de datos", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Login exitoso",
		"user":    user,
	})
}

func handleGetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	code := vars["code"]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var user User
	err := database.users.FindOne(ctx, bson.M{"code": code}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Error obteniendo usuario: %v", err)
		http.Error(w, "Error de base de datos", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	code := vars["code"]

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Error parseando formulario", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	lastName := r.FormValue("last_name")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"name":       name,
			"last_name":  lastName,
			"updated_at": time.Now(),
		},
	}

	file, header, err := r.FormFile("image")
	if err == nil {
		defer file.Close()

		ext := filepath.Ext(header.Filename)
		filename := fmt.Sprintf("%s%s", code, ext)
		filepath := filepath.Join("uploads", filename)

		dst, err := os.Create(filepath)
		if err != nil {
			http.Error(w, "Error guardando imagen", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		_, err = io.Copy(dst, file)
		if err != nil {
			http.Error(w, "Error guardando imagen", http.StatusInternalServerError)
			return
		}

		imageURL := fmt.Sprintf("http://localhost:8080/uploads/%s", filename)
		update["$set"].(bson.M)["image_url"] = imageURL
	}

	result, err := database.users.UpdateOne(
		ctx,
		bson.M{"code": code},
		update,
	)
	if err != nil {
		log.Printf("Error actualizando usuario: %v", err)
		http.Error(w, "Error actualizando usuario", http.StatusInternalServerError)
		return
	}

	if result.MatchedCount == 0 {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}

	var user User
	err = database.users.FindOne(ctx, bson.M{"code": code}).Decode(&user)
	if err != nil {
		log.Printf("Error obteniendo usuario actualizado: %v", err)
		http.Error(w, "Error obteniendo usuario", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Usuario actualizado correctamente",
		"user":    user,
	})
}
