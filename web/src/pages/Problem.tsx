import { useEffect, useState } from "react"
import { useParams, useNavigate } from "react-router-dom"
import { api, Problem as ProblemType, Submission, StatementMeta } from "../api"

export default function Problem() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const [problem, setProblem] = useState<ProblemType | null>(null)
  const [statement, setStatement] = useState("")
  const [availLangs, setAvailLangs] = useState<StatementMeta[]>([])
  const [activeLang, setActiveLang] = useState("en")
  const [code, setCode] = useState("")
  const [language, setLanguage] = useState("cpp17")
  const [submitting, setSubmitting] = useState(false)
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [err, setErr] = useState("")

  useEffect(() => {
    if (!statement) return
    ;(window as any).MathJax?.typesetPromise?.()
  }, [statement])

  useEffect(() => {
    if (!slug) return
    api.problem(slug, activeLang)
      .then(r => {
        setProblem(r.problem)
        setStatement(r.statement)
        setAvailLangs(r.available_langs ?? [])
      })
      .catch(e => setErr(e.message))
    api.submissions()
      .then(all => setSubmissions(all.filter(s => s.problem_slug === slug)))
      .catch(() => {})
  }, [slug, activeLang])

  async function onSubmit() {
    if (!slug || !code.trim()) return
    setSubmitting(true); setErr("")
    try {
      const { id } = await api.submit(slug, language, code)
      navigate(`/submissions/${id}`)
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "Submit failed")
      setSubmitting(false)
    }
  }

  if (err && !problem) return <p className="error">{err}</p>
  if (!problem) return <p>Loading…</p>

  return (
    <div className="page problem-page">
      <h2>{problem.title}</h2>
      <p className="limits">
        Time: {problem.time_limit}s &nbsp;|&nbsp; Memory: {problem.memory_limit}MB
        {availLangs.length > 1 && (
          <span className="lang-switcher">
            &nbsp;|&nbsp;
            {availLangs.map(l => (
              <button
                key={l.language}
                className={`lang-btn${activeLang === l.language ? " active" : ""}`}
                onClick={() => setActiveLang(l.language)}
              >
                {l.label}
              </button>
            ))}
          </span>
        )}
      </p>

      {statement && (
        <div
          className="statement"
          dangerouslySetInnerHTML={{ __html: statement }}
        />
      )}

      <div className="submit-box">
        <h3>Submit</h3>
        <div className="submit-controls">
          <select value={language} onChange={e => setLanguage(e.target.value)}>
            <option value="cpp17">C++17</option>
            <option value="cpp20">C++20</option>
            <option value="python3">Python 3</option>
          </select>
          <button onClick={onSubmit} disabled={submitting || !code.trim()}>
            {submitting ? "Submitting…" : "Submit"}
          </button>
        </div>
        <textarea
          rows={16}
          placeholder="Paste your code here…"
          value={code}
          onChange={e => setCode(e.target.value)}
        />
        {err && <p className="error">{err}</p>}
      </div>

      {submissions.length > 0 && (
        <div className="my-submissions">
          <h3>My Submissions</h3>
          <table className="table">
            <thead>
              <tr><th>#</th><th>Language</th><th>Verdict</th><th>Score</th><th>Time</th></tr>
            </thead>
            <tbody>
              {submissions.map(s => (
                <tr key={s.id}>
                  <td><a href={`/submissions/${s.id}`}>{s.id}</a></td>
                  <td>{s.language}</td>
                  <td className={`verdict verdict-${s.verdict}`}>{s.verdict || s.status}</td>
                  <td>{s.score}</td>
                  <td>{s.time_ms > 0 ? `${s.time_ms}ms` : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
