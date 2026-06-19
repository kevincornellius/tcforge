import { BrowserRouter, Routes, Route, Navigate, Link, useNavigate } from "react-router-dom"
import { AuthProvider, useAuth } from "./auth"
import Login from "./pages/Login"
import Problems from "./pages/Problems"
import Problem from "./pages/Problem"
import Submission from "./pages/Submission"
import Scoreboard from "./pages/Scoreboard"
import Admin from "./pages/Admin"

function Nav() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  function handleLogout() {
    logout()
    navigate("/login")
  }

  if (!user) return null

  return (
    <nav>
      <span className="nav-brand">tcforge</span>
      <div className="nav-links">
        <Link to="/problems">Problems</Link>
        <Link to="/scoreboard">Scoreboard</Link>
        {user.is_admin && <Link to="/admin">Admin</Link>}
      </div>
      <div className="nav-user">
        <span>{user.display_name}</span>
        <button onClick={handleLogout}>Logout</button>
      </div>
    </nav>
  )
}

function RequireAuth({ children }: { children: JSX.Element }) {
  const { user } = useAuth()
  if (!user && !localStorage.getItem("token")) {
    return <Navigate to="/login" replace />
  }
  return children
}

function AppRoutes() {
  return (
    <>
      <Nav />
      <main>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/problems" element={<RequireAuth><Problems /></RequireAuth>} />
          <Route path="/problems/:slug" element={<RequireAuth><Problem /></RequireAuth>} />
          <Route path="/submissions/:id" element={<RequireAuth><Submission /></RequireAuth>} />
          <Route path="/scoreboard" element={<RequireAuth><Scoreboard /></RequireAuth>} />
          <Route path="/admin" element={<RequireAuth><Admin /></RequireAuth>} />
          <Route path="*" element={<Navigate to="/problems" replace />} />
        </Routes>
      </main>
    </>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </AuthProvider>
  )
}
