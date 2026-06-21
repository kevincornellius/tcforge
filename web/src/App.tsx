import { BrowserRouter, Routes, Route, Navigate, NavLink, useNavigate, useLocation } from "react-router-dom"
import { AuthProvider, useAuth } from "./auth"
import { createContext, useContext, useEffect, useRef, useState } from "react"
import { api, ContestState } from "./api"

export const ContestContext = createContext<ContestState | null>(null)
export function useContest() { return useContext(ContestContext) }

import Login from "./pages/Login"
import Problems from "./pages/Problems"
import Problem from "./pages/Problem"
import Submission from "./pages/Submission"
import Scoreboard from "./pages/Scoreboard"
import Admin from "./pages/Admin"
import Announcements from "./pages/Announcements"

function useCountdown(endAt: string | null): string {
  const [display, setDisplay] = useState("")

  useEffect(() => {
    if (!endAt) { setDisplay(""); return }

    function update() {
      const diff = new Date(endAt!).getTime() - Date.now()
      if (diff <= 0) { setDisplay("Ended"); return }
      const h = Math.floor(diff / 3600000)
      const m = Math.floor((diff % 3600000) / 60000)
      const s = Math.floor((diff % 60000) / 1000)
      setDisplay(`${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`)
    }

    update()
    const id = setInterval(update, 1000)
    return () => clearInterval(id)
  }, [endAt])

  return display
}

function Nav({ contest }: { contest: ContestState | null }) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const countdown = useCountdown(contest?.end_at ?? null)
  const [hidden, setHidden] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const lastScrollY = useRef(0)

  const isSlim = hidden && !menuOpen

  useEffect(() => {
    setHidden(false)
    setMenuOpen(false)
    lastScrollY.current = 0
  }, [location.pathname])

  useEffect(() => {
    function onScroll() {
      const y = window.scrollY
      if (y < 60) { setHidden(false); lastScrollY.current = y; return }
      setHidden(y > lastScrollY.current)
      lastScrollY.current = y
    }
    window.addEventListener("scroll", onScroll, { passive: true })
    return () => window.removeEventListener("scroll", onScroll)
  }, [])

  function handleLogout() { logout(); navigate("/login") }

  if (!user) return null

  const now = Date.now()
  const running = contest?.start_at && new Date(contest.start_at).getTime() <= now
    && (!contest.end_at || new Date(contest.end_at).getTime() > now)

  return (
    <div className={`nav-shell${isSlim ? " nav-shell-hidden" : ""}`}>
      <nav className={menuOpen ? "nav-mobile-open" : ""}>
        <div className="nav-logo">
          <img src="/logo.svg" alt="tcforge" />
          <span className="nav-brand">{contest?.name || "tcforge"}</span>
        </div>

        <button
          className="nav-hamburger"
          onClick={() => setMenuOpen(m => !m)}
          aria-label="Toggle menu"
        >
          {menuOpen ? "✕" : "☰"}
        </button>

        <div className="nav-expandable">
          <div className="nav-sep" />

          <div className="nav-links">
            <NavLink to="/problems"      className={({ isActive }) => isActive ? "active" : ""}>Problems</NavLink>
            <NavLink to="/scoreboard"    className={({ isActive }) => isActive ? "active" : ""}>Scoreboard</NavLink>
            <NavLink to="/announcements" className={({ isActive }) => isActive ? "active" : ""}>Announce</NavLink>
            {user.is_admin && <NavLink to="/admin" className={({ isActive }) => isActive ? "active" : ""}>Admin</NavLink>}
          </div>

          <div className="nav-right">
            {countdown && (
              <span className={`nav-countdown${running ? "" : " ended"}`}>
                {running ? `⏱ ${countdown}` : countdown}
              </span>
            )}
            <span className="nav-user-name">{user.display_name}</span>
            <button className="nav-logout" onClick={handleLogout}>Logout</button>
          </div>
        </div>
      </nav>
    </div>
  )
}

function RequireAuth({ children }: { children: JSX.Element }) {
  const { user } = useAuth()
  if (!user && !localStorage.getItem("token")) {
    return <Navigate to="/login" replace />
  }
  return children
}

function ContestGate({ children, contest }: { children: JSX.Element; contest: ContestState | null }) {
  const { user } = useAuth()
  if (!contest || user?.is_admin) return children
  if (contest.always_open) return children

  const now = Date.now()

  if (!contest.start_at) {
    return (
      <div className="gate-wrap">
        <div className="gate-card">
          <h2 className="gate-title">Contest not yet published</h2>
          <p className="gate-desc">Check back later.</p>
        </div>
      </div>
    )
  }
  if (new Date(contest.start_at).getTime() > now) {
    return (
      <div className="gate-wrap">
        <div className="gate-card">
          <h2 className="gate-title">Contest hasn't started yet</h2>
          <p className="gate-desc">Starts at <strong>{new Date(contest.start_at).toLocaleString()}</strong></p>
        </div>
      </div>
    )
  }
  if (contest.end_at && new Date(contest.end_at).getTime() < now) {
    return (
      <div className="gate-wrap">
        <div className="gate-card">
          <h2 className="gate-title">Contest has ended</h2>
          <p className="gate-desc">Submissions are closed.</p>
        </div>
      </div>
    )
  }
  return children
}

function AppRoutes() {
  const [contest, setContest] = useState<ContestState | null>(null)
  const { user } = useAuth()

  useEffect(() => {
    if (!user) return
    api.contest().then(setContest).catch(() => {})
    const id = setInterval(() => api.contest().then(setContest).catch(() => {}), 30000)
    return () => clearInterval(id)
  }, [user])

  const gate = (el: JSX.Element) => (
    <RequireAuth><ContestGate contest={contest}>{el}</ContestGate></RequireAuth>
  )

  return (
    <ContestContext.Provider value={contest}>
      <Nav contest={contest} />
      <main>
        <Routes>
          <Route path="/login"            element={<Login />} />
          <Route path="/problems"         element={gate(<Problems />)} />
          <Route path="/problems/:slug"   element={gate(<Problem />)} />
          <Route path="/submissions/:id"  element={gate(<Submission />)} />
          <Route path="/scoreboard"       element={gate(<Scoreboard />)} />
          <Route path="/announcements"    element={<RequireAuth><Announcements /></RequireAuth>} />
          <Route path="/admin"            element={<RequireAuth><Admin /></RequireAuth>} />
          <Route path="*"                 element={<Navigate to="/problems" replace />} />
        </Routes>
      </main>
      <footer className="site-footer">
        <span>{contest?.name || "tcforge"}</span>
        <a href="https://github.com/kevincornellius/tcforge" target="_blank" rel="noopener noreferrer" className="site-footer-link">powered by TCForge</a>
      </footer>
    </ContestContext.Provider>
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
