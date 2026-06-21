import { useState, FormEvent } from "react"
import { useNavigate } from "react-router-dom"
import { useAuth } from "../auth"

export default function Login() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [err, setErr] = useState("")
  const [loading, setLoading] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setErr("")
    setLoading(true)
    try {
      await login(username, password)
      navigate("/problems")
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "Login failed")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="login-logo-wrap">
          <img src="/logo_text.svg" alt="tcforge" />
          <span className="login-sub">Contest Platform</span>
        </div>
        <form className="login-form" onSubmit={onSubmit}>
          <input
            className="input-field"
            placeholder="Username"
            value={username}
            onChange={e => setUsername(e.target.value)}
            autoFocus
            required
          />
          <input
            className="input-field"
            type="password"
            placeholder="Password"
            value={password}
            onChange={e => setPassword(e.target.value)}
            required
          />
          {err && <p className="error-msg">{err}</p>}
          <button className="btn btn-primary btn-lg btn-full" type="submit" disabled={loading}>
            {loading ? "Signing in…" : "Sign in"}
          </button>
        </form>
      </div>
    </div>
  )
}
