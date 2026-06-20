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
    <div className="page">
      <h2>Announcements</h2>
      {err && <p className="error">{err}</p>}
      {items.length === 0 && <p style={{ color: "#888" }}>No announcements.</p>}
      <div className="announce-list">
        {items.map(a => (
          <div key={a.id} className="announce-item">
            <span className="announce-time">{new Date(a.created_at).toLocaleString()}</span>
            <p className="announce-msg">{a.message}</p>
          </div>
        ))}
      </div>
    </div>
  )
}
