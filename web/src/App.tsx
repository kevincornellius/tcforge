import { BrowserRouter, Routes, Route, Navigate, Link, useNavigate } from "react-router-dom"
import { AuthProvider, useAuth } from "./auth"
import { useEffect, useState } from "react"
import { api, ContestState } from "./api"
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

function Nav({ contest, setContest }: { contest: ContestState | null; setContest: (c: ContestState) => void }) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  void setContest // consumed by AppRoutes; passed here only so Nav can trigger refresh if needed

  const countdown = useCountdown(contest?.end_at ?? null)

  function handleLogout() { logout(); navigate("/login") }

  if (!user) return null

  const now = Date.now()
  const running = contest?.start_at && new Date(contest.start_at).getTime() <= now
    && (!contest.end_at || new Date(contest.end_at).getTime() > now)

  return (
    <nav>
      <span className="nav-brand">{contest?.name || "tcforge"}</span>
      <div className="nav-links">
        <Link to="/problems">Problems</Link>
        <Link to="/scoreboard">Scoreboard</Link>
        <Link to="/announcements">Announcements</Link>
        {user.is_admin && <Link to="/admin">Admin</Link>}
      </div>
      {countdown && (
        <span className={`nav-countdown ${running ? "" : "ended"}`}>
          {running ? `⏱ ${countdown}` : countdown}
        </span>
      )}
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

function ContestGate({ children, contest }: { children: JSX.Element; contest: ContestState | null }) {
  const { user } = useAuth()
  if (!contest || user?.is_admin) return children

  if (contest.always_open) return children

  const now = Date.now()
  if (contest.start_at && new Date(contest.start_at).getTime() > now) {
    const start = new Date(contest.start_at)
    return (
      <div className="contest-gate">
        <div className="contest-gate-box">
          <h2>Contest hasn't started yet</h2>
          <p>Starts at <strong>{start.toLocaleString()}</strong></p>
        </div>
      </div>
    )
  }
  if (contest.end_at && new Date(contest.end_at).getTime() < now) {
    return (
      <div className="contest-gate">
        <div className="contest-gate-box">
          <h2>Contest has ended</h2>
          <p>Submissions are closed.</p>
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
    <>
      <Nav contest={contest} setContest={setContest} />
      <main>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/problems" element={gate(<Problems />)} />
          <Route path="/problems/:slug" element={gate(<Problem />)} />
          <Route path="/submissions/:id" element={gate(<Submission />)} />
          <Route path="/scoreboard" element={gate(<Scoreboard />)} />
          <Route path="/announcements" element={<RequireAuth><Announcements /></RequireAuth>} />
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
