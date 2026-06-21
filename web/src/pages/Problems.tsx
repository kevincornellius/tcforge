import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { api, Problem, Submission } from "../api"

function positionLetter(pos: number): string {
  return String.fromCharCode(65 + pos)
}

export default function Problems() {
  const [problems, setProblems] = useState<Problem[]>([])
  const [bestBySlug, setBestBySlug] = useState<Map<string, Submission>>(new Map())
  const [err, setErr] = useState("")

  useEffect(() => {
    Promise.all([api.problems(), api.submissions()])
      .then(([probs, subs]) => {
        setProblems(probs)
        const best = new Map<string, Submission>()
        for (const s of subs) {
          const prev = best.get(s.problem_slug)
          if (!prev || s.score > prev.score) best.set(s.problem_slug, s)
        }
        setBestBySlug(best)
      })
      .catch(e => setErr(e.message))
  }, [])

  if (err) return <p className="error-msg">{err}</p>

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Problems</h1>
      </div>

      <div className="problems-list">
        <div className="problems-header">
          <span>#</span>
          <span>Title</span>
          <span>Time</span>
          <span>Memory</span>
          <span>Score</span>
        </div>

        {problems.length === 0 && (
          <div style={{ padding: "2rem 1.25rem", color: "var(--color-muted)", fontSize: "0.9rem" }}>
            No problems yet.
          </div>
        )}

        {problems.map(p => {
          const best = bestBySlug.get(p.slug)
          return (
            <Link
              key={p.slug}
              to={`/problems/${p.slug}`}
              className={`problem-row${best ? ` prob-attempted${best.verdict === "AC" ? " prob-ac" : ""}` : ""}`}
            >
              <span className="prob-letter">{positionLetter(p.position)}</span>
              <span className="prob-title">{p.title}</span>
              <span className="prob-limit">{p.time_limit}s</span>
              <span className="prob-limit">{p.memory_limit}MB</span>
              <span className="prob-score">
                {best
                  ? <span className={`badge badge-${best.verdict}`}>{best.score > 0 ? best.score : best.verdict}</span>
                  : <span className="prob-score-empty">—</span>}
              </span>
            </Link>
          )
        })}
      </div>
    </div>
  )
}
