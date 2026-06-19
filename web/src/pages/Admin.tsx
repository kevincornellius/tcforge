import { useEffect, useState, FormEvent } from "react"
import { api, AdminUser } from "../api"

export default function Admin() {
  const [users, setUsers] = useState<AdminUser[]>([])
  const [err, setErr] = useState("")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [isAdmin, setIsAdmin] = useState(false)
  const [adding, setAdding] = useState(false)

  // reset password state
  const [resetId, setResetId] = useState<number | null>(null)
  const [resetPw, setResetPw] = useState("")

  function load() {
    api.admin.users().then(setUsers).catch(e => setErr(e.message))
  }

  useEffect(() => { load() }, [])

  async function onAdd(e: FormEvent) {
    e.preventDefault()
    setErr("")
    setAdding(true)
    try {
      await api.admin.createUser(username, password, displayName || username, isAdmin)
      setUsername(""); setPassword(""); setDisplayName(""); setIsAdmin(false)
      load()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "Failed")
    } finally {
      setAdding(false)
    }
  }

  async function onDelete(id: number) {
    if (!confirm("Delete this user?")) return
    await api.admin.deleteUser(id).catch(e => setErr(e.message))
    load()
  }

  async function onResetPassword(id: number) {
    if (!resetPw.trim()) return
    await api.admin.resetPassword(id, resetPw).catch(e => setErr(e.message))
    setResetId(null); setResetPw("")
  }

  return (
    <div className="page admin-page">
      <h2>Admin — Users</h2>
      {err && <p className="error">{err}</p>}

      <table className="table">
        <thead>
          <tr>
            <th>#</th>
            <th>Username</th>
            <th>Display name</th>
            <th>Admin</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {users.map(u => (
            <tr key={u.id}>
              <td>{u.id}</td>
              <td>{u.username}</td>
              <td>{u.display_name}</td>
              <td>{u.is_admin ? "yes" : "—"}</td>
              <td className="admin-actions">
                {resetId === u.id ? (
                  <>
                    <input
                      type="password"
                      placeholder="New password"
                      value={resetPw}
                      onChange={e => setResetPw(e.target.value)}
                    />
                    <button onClick={() => onResetPassword(u.id)}>Save</button>
                    <button className="btn-ghost" onClick={() => setResetId(null)}>Cancel</button>
                  </>
                ) : (
                  <>
                    <button className="btn-ghost" onClick={() => { setResetId(u.id); setResetPw("") }}>
                      Reset pw
                    </button>
                    <button className="btn-danger" onClick={() => onDelete(u.id)}>Delete</button>
                  </>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <div className="admin-add">
        <h3>Add user</h3>
        <form onSubmit={onAdd}>
          <input placeholder="Username" value={username} onChange={e => setUsername(e.target.value)} required />
          <input placeholder="Display name (optional)" value={displayName} onChange={e => setDisplayName(e.target.value)} />
          <input type="password" placeholder="Password" value={password} onChange={e => setPassword(e.target.value)} required />
          <label className="admin-checkbox">
            <input type="checkbox" checked={isAdmin} onChange={e => setIsAdmin(e.target.checked)} />
            Admin
          </label>
          <button type="submit" disabled={adding}>{adding ? "Adding…" : "Add user"}</button>
        </form>
      </div>
    </div>
  )
}
