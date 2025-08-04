import React, { useState, useEffect } from 'react';
import './App.css';

const API_BASE_URL = 'http://localhost:8080';

function App() {
  const [currentView, setCurrentView] = useState('register');
  const [userCode, setUserCode] = useState('');
  const [user, setUser] = useState(null);

  useEffect(() => {
    const savedCode = localStorage.getItem('userCode');
    if (savedCode) {
      setUserCode(savedCode);
      setCurrentView('profile');
      fetchUserProfile(savedCode);
    }
  }, []);

  const fetchUserProfile = async (code) => {
    try {
      const response = await fetch(`${API_BASE_URL}/api/user/${code}`);
      if (response.ok) {
        const userData = await response.json();
        setUser(userData);
      }
    } catch (error) {
      console.error('Error fetching user profile:', error);
    }
  };

  const RegisterView = () => {
    const [email, setEmail] = useState('');
    const [loading, setLoading] = useState(false);
    const [message, setMessage] = useState('');

    const handleRegister = async (e) => {
      e.preventDefault();
      setLoading(true);
      setMessage('');

      try {
        const response = await fetch(`${API_BASE_URL}/api/register`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ email }),
        });

        const data = await response.json();

        if (response.ok) {
          setMessage(`Código enviado a ${email}. Revisa tu correo.`);
          setTimeout(() => {
            setCurrentView('login');
          }, 2000);
        } else {
          setMessage(data.error || 'Error en el registro');
        }
      } catch (error) {
        setMessage('Error de conexión');
      } finally {
        setLoading(false);
      }
    };

    return (
      <div className="view-container">
        <div className="form-card">
          <h2>Registro</h2>
          <form onSubmit={handleRegister}>
            <div className="form-group">
              <label htmlFor="email">Correo Electrónico:</label>
              <input
                type="email"
                id="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                placeholder="tu@email.com"
              />
            </div>
            <button type="submit" disabled={loading} className="btn-primary">
              {loading ? 'Enviando...' : 'Registrar'}
            </button>
          </form>
          {message && <div className="message">{message}</div>}
          <p className="link" onClick={() => setCurrentView('login')}>
            ¿Ya tienes un código? Inicia sesión
          </p>
        </div>
      </div>
    );
  };

  const LoginView = () => {
    const [code, setCode] = useState('');
    const [loading, setLoading] = useState(false);
    const [message, setMessage] = useState('');

    const handleLogin = async (e) => {
      e.preventDefault();
      setLoading(true);
      setMessage('');

      try {
        const response = await fetch(`${API_BASE_URL}/api/login`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ code }),
        });

        const data = await response.json();

        if (response.ok) {
          setUserCode(code);
          localStorage.setItem('userCode', code);
          setUser(data.user);
          setCurrentView('profile');
        } else {
          setMessage(data.error || 'Código inválido');
        }
      } catch (error) {
        setMessage('Error de conexión');
      } finally {
        setLoading(false);
      }
    };

    return (
      <div className="view-container">
        <div className="form-card">
          <h2>Iniciar Sesión</h2>
          <form onSubmit={handleLogin}>
            <div className="form-group">
              <label htmlFor="code">Código:</label>
              <input
                type="text"
                id="code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                required
                placeholder="A01-1"
              />
            </div>
            <button type="submit" disabled={loading} className="btn-primary">
              {loading ? 'Iniciando...' : 'Iniciar Sesión'}
            </button>
          </form>
          {message && <div className="message error">{message}</div>}
          <p className="link" onClick={() => setCurrentView('register')}>
            ¿No tienes código? Regístrate
          </p>
        </div>
      </div>
    );
  };

  const ProfileView = () => {
    const [name, setName] = useState(user?.name || '');
    const [lastName, setLastName] = useState(user?.last_name || '');
    const [image, setImage] = useState(null);
    const [imagePreview, setImagePreview] = useState(user?.image_url || '');
    const [loading, setLoading] = useState(false);
    const [message, setMessage] = useState('');

    const handleImageChange = (e) => {
      const file = e.target.files[0];
      if (file) {
        setImage(file);
        const reader = new FileReader();
        reader.onloadend = () => {
          setImagePreview(reader.result);
        };
        reader.readAsDataURL(file);
      }
    };

    const handleUpdateProfile = async (e) => {
      e.preventDefault();
      setLoading(true);
      setMessage('');

      const formData = new FormData();
      formData.append('name', name);
      formData.append('last_name', lastName);
      if (image) {
        formData.append('image', image);
      }

      try {
        const response = await fetch(`${API_BASE_URL}/api/user/${userCode}`, {
          method: 'PUT',
          body: formData,
        });

        const data = await response.json();

        if (response.ok) {
          setUser(data.user);
          setMessage('Perfil actualizado correctamente');
        } else {
          setMessage(data.error || 'Error al actualizar');
        }
      } catch (error) {
        setMessage('Error de conexión');
      } finally {
        setLoading(false);
      }
    };

    const handleLogout = () => {
      localStorage.removeItem('userCode');
      setUserCode('');
      setUser(null);
      setCurrentView('register');
    };

    return (
      <div className="view-container">
        <div className="profile-card">
          <div className="profile-header">
            <h2>Mi Perfil</h2>
            <div className="user-code">Código: {userCode}</div>
            <button onClick={handleLogout} className="btn-logout">
              Cerrar Sesión
            </button>
          </div>

          <form onSubmit={handleUpdateProfile}>
            <div className="profile-image-section">
              <div className="image-preview">
                {imagePreview ? (
                  <img src={imagePreview} alt="Perfil" />
                ) : (
                  <div className="no-image">Sin imagen</div>
                )}
              </div>
              <input
                type="file"
                accept="image/*"
                onChange={handleImageChange}
                className="file-input"
              />
            </div>

            <div className="form-group">
              <label htmlFor="name">Nombre:</label>
              <input
                type="text"
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Tu nombre"
              />
            </div>

            <div className="form-group">
              <label htmlFor="lastName">Apellidos:</label>
              <input
                type="text"
                id="lastName"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                placeholder="Tus apellidos"
              />
            </div>

            <div className="form-group">
              <label htmlFor="name">Direccion:</label>
              <input
                type="text"
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Tu nombre"
              />
            </div>

            <div className="form-group">
              <label htmlFor="number">Telfono:</label>
              <input
                type="text"
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Tu nombre"
              />
            </div>


            <button type="submit" disabled={loading} className="btn-primary">
              {loading ? 'Guardando...' : 'Guardar Cambios'}
            </button>
          </form>

          {message && <div className="message">{message}</div>}
        </div>
      </div>
    );
  };

  return (
    <div className="App">
      {currentView === 'register' && <RegisterView />}
      {currentView === 'login' && <LoginView />}
      {currentView === 'profile' && <ProfileView />}
    </div>
  );
}

export default App;