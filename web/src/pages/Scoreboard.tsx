import { useEffect, useState } from "react"
import { api, ScoreboardEntry, Problem } from "../api"

export default function Scoreboard() {
  const [entries, setEntries] = useState<ScoreboardEntry[]>([])
  const [problems, setProblems] = useState<Problem[]>([])
  const [err, setErr] = useState("")

  useEffect(() => {
    Promise.all([api.scoreboard(), api.problems()])
      .then(([e, p]) => { setEntries(e); setProblems(p) })
      .catch(e => setErr(e.message))
  }, [])

  if (err) return <p className="error">{err}</p>

  return (
    <div className="page">
      <h2>Scoreboard</h2>
      <table className="table scoreboard">
        <thead>
          <tr>
            <th>Rank</th>
            <th>Participant</th>
            {problems.map(p => <th key={p.slug}>{p.title}</th>)}
            <th>Total</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((e, i) => (
            <tr key={e.user_id}>
              <td>{i + 1}</td>
              <td>{e.display_name}</td>
              {problems.map(p => (
                <td key={p.slug}>{e.problems[p.slug] ?? 0}</td>
              ))}
              <td><strong>{e.total_score}</strong></td>
            </tr>
          ))}
          {entries.length === 0 && (
            <tr><td colSpan={problems.length + 3}>No submissions yet.</td></tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
