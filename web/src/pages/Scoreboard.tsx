import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { api, ScoreboardEntry, ScoreboardData, Problem } from "../api"

function rankClass(i: number): string {
  if (i === 0) return "rank-1"
  if (i === 1) return "rank-2"
  if (i === 2) return "rank-3"
  return ""
}

function rankLabel(i: number): string {
  if (i === 0) return "🥇"
  if (i === 1) return "🥈"
  if (i === 2) return "🥉"
  return String(i + 1)
}

export default function Scoreboard() {
  const [data, setData] = useState<ScoreboardData | null>(null)
  const [problems, setProblems] = useState<Problem[]>([])
  const [err, setErr] = useState("")

  useEffect(() => {
    Promise.all([api.scoreboard(), api.problems()])
      .then(([d, p]) => { setData(d); setProblems(p) })
      .catch(e => setErr(e.message))
  }, [])

  if (err) return <p className="error-msg">{err}</p>

  const entries = data?.entries ?? []
  const firstSolvers = data?.first_solvers ?? {}
  const problemMaxScore = data?.problem_max_score ?? {}

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Scoreboard</h1>
      </div>

      <div className="table-wrap">
        <table className="table">
          <thead>
            <tr>
              <th style={{ width: 52 }}>#</th>
              <th>Participant</th>
              {problems.map(p => (
                <th key={p.slug} className="center" style={{ minWidth: 72 }}>
                  <Link to={`/problems/${p.slug}`} className="prob-col-link">
                    {String.fromCharCode(65 + p.position)}
                  </Link>
                </th>
              ))}
              <th className="center" style={{ minWidth: 72 }}>Total</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e, i) => (
              <tr key={e.user_id}>
                <td>
                  <span className={`rank-num ${rankClass(i)}`}>{rankLabel(i)}</span>
                </td>
                <td className="participant-name">{e.display_name}</td>
                {problems.map(p => {
                  const score = e.problems[p.slug] ?? 0
                  const maxScore = problemMaxScore[p.slug] ?? 100
                  const isAC = score > 0 && score >= maxScore
                  const isFirst = isAC && firstSolvers[p.slug] === e.user_id
                  const cls = isFirst
                    ? "score-val score-first"
                    : isAC
                    ? "score-val score-ac"
                    : score === 0
                    ? "score-val score-zero"
                    : "score-val"
                  return (
                    <td key={p.slug} className={cls}>
                      {score || ""}
                    </td>
                  )
                })}
                <td className="total-val">{e.total_score}</td>
              </tr>
            ))}
            {entries.length === 0 && (
              <tr>
                <td colSpan={problems.length + 3} className="muted-msg" style={{ textAlign: "center", padding: "2rem" }}>
                  No submissions yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
