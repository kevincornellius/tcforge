import { useEffect, useState } from "react"
import { api, Announcement } from "../api"

export default function Announcements() {
  const [items, setItems] = useState<Announcement[]>([])
  const [err, setErr] = useState("")

  useEffect(() => {
    api.announcements().then(setItems).catch(e => setErr(e.message))
    const id = setInterval(() => api.announcements().then(setItems).catch(() => {}), 30000)
    return () => clearInterval(id)
  }, [])

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Announcements</h1>
      </div>

      {err && <p className="error-msg">{err}</p>}

      {items.length === 0 ? (
        <p className="muted-msg">No announcements yet.</p>
      ) : (
        <div className="announce-list">
          {items.map(a => (
            <div key={a.id} className="announce-card">
              <span className="announce-time">{new Date(a.created_at).toLocaleString()}</span>
              <p className="announce-msg">{a.message}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
