import { useEffect, useState } from "react"
import { useParams } from "react-router-dom"
import { api, Submission as Sub, Verdict } from "../api"

export default function Submission() {
  const { id } = useParams<{ id: string }>()
  const [sub, setSub] = useState<Sub | null>(null)
  const [verdicts, setVerdicts] = useState<Verdict[]>([])
  const [err, setErr] = useState("")

  useEffect(() => {
    if (!id) return
    const numId = parseInt(id, 10)
    const load = () =>
      api.submission(numId).then(r => { setSub(r.submission); setVerdicts(r.verdicts) })
    load().catch(e => setErr(e.message))

    // poll while pending
    const interval = setInterval(() => {
      api.submission(numId).then(r => {
        setSub(r.submission)
        setVerdicts(r.verdicts)
        if (r.submission.status !== "pending" && r.submission.status !== "judging") {
          clearInterval(interval)
        }
      }).catch(() => clearInterval(interval))
    }, 1500)
    return () => clearInterval(interval)
  }, [id])

  if (err) return <p className="error">{err}</p>
  if (!sub) return <p>Loading…</p>

  return (
    <div className="page">
      <h2>Submission #{sub.id}</h2>
      <p>
        <strong>Problem:</strong> {sub.problem_title} &nbsp;|&nbsp;
        <strong>Language:</strong> {sub.language} &nbsp;|&nbsp;
        <strong>Status:</strong> <span className={`verdict verdict-${sub.verdict}`}>{sub.verdict || sub.status}</span>
        &nbsp;|&nbsp;
        <strong>Score:</strong> {sub.score}
      </p>

      {verdicts.length > 0 && (
        <table className="table">
          <thead>
            <tr>
              <th>Test case</th>
              <th>Verdict</th>
              <th>Time</th>
              <th>Memory</th>
            </tr>
          </thead>
          <tbody>
            {verdicts.map(v => (
              <tr key={v.test_case}>
                <td>{v.test_case}</td>
                <td className={`verdict verdict-${v.verdict}`}>{v.verdict}</td>
                <td>{v.time_ms > 0 ? `${v.time_ms}ms` : "—"}</td>
                <td>{v.memory_kb > 0 ? `${v.memory_kb}KB` : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {(sub.status === "pending" || sub.status === "judging") && (
        <p className="judging-status">Judging in progress…</p>
      )}
    </div>
  )
}
