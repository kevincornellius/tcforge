import { useEffect, useRef, useState, ChangeEvent } from "react"
import { useParams, useNavigate, Link } from "react-router-dom"
import { api, Problem as ProblemType, Submission, StatementMeta } from "../api"
import { useContest } from "../App"

function VerdictBadge({ verdict, status }: { verdict: string; status: string }) {
  const v = verdict || status
  return <span className={`badge badge-${v}`}>{v}</span>
}

function fmtLang(lang: string): string {
  const map: Record<string, string> = { cpp17: "C++17", cpp20: "C++20", python3: "Python 3" }
  return map[lang] ?? lang
}

function MySubList({ submissions }: { submissions: Submission[] }) {
  const navigate = useNavigate()
  return (
    <div className="my-sub-list">
      <div className="my-sub-header">
        <span>Verdict</span>
        <span>Score</span>
        <span>Lang</span>
        <span>Time</span>
        <span>Mem</span>
      </div>
      {submissions.map(s => (
        <div
          key={s.id}
          className="my-sub-row"
          onClick={() => navigate(`/submissions/${s.id}`)}
          role="button"
          tabIndex={0}
          onKeyDown={e => e.key === "Enter" && navigate(`/submissions/${s.id}`)}
        >
          <span><VerdictBadge verdict={s.verdict} status={s.status} /></span>
          <span className="mono-label">{s.score}</span>
          <span className="mono-label">{fmtLang(s.language)}</span>
          <span className="mono-label">{s.time_ms > 0 ? `${s.time_ms}ms` : "—"}</span>
          <span className="mono-label">{s.memory_kb > 0 ? `${s.memory_kb}KB` : "—"}</span>
        </div>
      ))}
    </div>
  )
}

export default function Problem() {
  const { slug } = useParams<{ slug: string }>()
  const contest = useContest()
  const canSubmit = !contest || contest.always_open ? (contest?.allow_submission ?? true) : true

  const statementRef = useRef<HTMLDivElement>(null)
  const [problem, setProblem] = useState<ProblemType | null>(null)
  const [statement, setStatement] = useState("")
  const [availLangs, setAvailLangs] = useState<StatementMeta[]>([])
  const [activeLang, setActiveLang] = useState("en")
  const [code, setCode] = useState("")
  const [language, setLanguage] = useState("cpp17")
  const [uploadedFileName, setUploadedFileName] = useState("")
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [submitting, setSubmitting] = useState(false)
  const [submissions, setSubmissions] = useState<Submission[]>([])
  const [err, setErr] = useState("")
  const [sideTab, setSideTab] = useState<"submit" | "history">("submit")

  useEffect(() => {
    if (!statement) return
    ;(window as any).MathJax?.typesetPromise?.()
  }, [statement])

  useEffect(() => {
    const el = statementRef.current
    if (!el || !statement) return
    el.querySelectorAll<HTMLPreElement>("pre").forEach(pre => {
      if (pre.dataset.copyBtn) return
      pre.dataset.copyBtn = "1"
      const btn = document.createElement("button")
      btn.className = "pre-copy-btn"
      btn.textContent = "Copy"
      btn.addEventListener("click", () => {
        navigator.clipboard.writeText(pre.innerText.replace(/\nCopy$/, "")).then(() => {
          btn.textContent = "Copied!"
          setTimeout(() => { btn.textContent = "Copy" }, 2000)
        })
      })
      pre.appendChild(btn)
    })
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

  async function onFileUpload(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    const text = await file.text()
    setCode(text)
    setUploadedFileName(file.name)
    const name = file.name.toLowerCase()
    if (name.endsWith(".py")) setLanguage("python3")
    else if (name.endsWith(".cpp") || name.endsWith(".cc") || name.endsWith(".cxx")) setLanguage("cpp17")
  }

  function clearFile() {
    setUploadedFileName("")
    if (fileInputRef.current) fileInputRef.current.value = ""
  }

  async function onSubmit() {
    if (!slug || !code.trim()) return
    setSubmitting(true); setErr("")
    try {
      await api.submit(slug, language, code)
      const all = await api.submissions()
      setSubmissions(all.filter(s => s.problem_slug === slug))
      setSideTab("history")
      setSubmitting(false)
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "Submit failed")
      setSubmitting(false)
    }
  }

  if (err && !problem) return <p className="error-msg">{err}</p>
  if (!problem) return <p className="muted-msg">Loading…</p>

  return (
    <div>
      <div className="problem-header">
        <h1 className="page-title">{problem.title}</h1>
        <div className="problem-meta">
          <div className="limit-chips">
            <span className="limit-chip">⏱ {problem.time_limit}s</span>
            <span className="limit-chip">💾 {problem.memory_limit}MB</span>
          </div>
          {availLangs.length > 1 && (
            <div className="lang-switcher">
              {availLangs.map(l => (
                <button
                  key={l.language}
                  className={`lang-btn${activeLang === l.language ? " active" : ""}`}
                  onClick={() => setActiveLang(l.language)}
                >
                  {l.label}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="problem-layout">
        {/* Left: statement */}
        <div className="problem-main-col">
          {statement && (
            <div
              ref={statementRef}
              className="statement-card"
              dangerouslySetInnerHTML={{ __html: statement }}
            />
          )}
        </div>

        {/* Right: sticky tabbed panel */}
        <div className="problem-submit-col">
          <div className="submit-panel">
            <div className="side-tabs">
              <button
                className={`side-tab${sideTab === "submit" ? " active" : ""}`}
                onClick={() => setSideTab("submit")}
              >Submit</button>
              <button
                className={`side-tab${sideTab === "history" ? " active" : ""}`}
                onClick={() => setSideTab("history")}
              >
                My submissions
                {submissions.length > 0 && <span className="side-tab-count">{submissions.length}</span>}
              </button>
            </div>

            {sideTab === "submit" ? (
              <div className="submit-panel-body">
                <select
                  className="select-field"
                  value={language}
                  onChange={e => setLanguage(e.target.value)}
                >
                  <option value="cpp17">C++17</option>
                  <option value="cpp20">C++20</option>
                  <option value="python3">Python 3</option>
                </select>

                <div className="file-upload-row">
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept=".cpp,.cc,.cxx,.py"
                    style={{ display: "none" }}
                    onChange={onFileUpload}
                  />
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => fileInputRef.current?.click()}
                  >
                    Upload file
                  </button>
                  {uploadedFileName && (
                    <>
                      <span className="file-name-chip">{uploadedFileName}</span>
                      <button
                        type="button"
                        className="btn btn-ghost btn-xs"
                        onClick={clearFile}
                        aria-label="Clear file"
                      >✕</button>
                    </>
                  )}
                </div>

                <textarea
                  className="code-editor"
                  rows={14}
                  placeholder="Paste your code here…"
                  value={code}
                  onChange={e => { setCode(e.target.value); setUploadedFileName("") }}
                  spellCheck={false}
                />

                {err && <p className="error-msg">{err}</p>}

                <button
                  className="btn btn-primary btn-md btn-full"
                  onClick={onSubmit}
                  disabled={submitting || !code.trim() || !canSubmit}
                >
                  {submitting ? "Submitting…" : !canSubmit ? "Submissions disabled" : "Submit"}
                </button>
              </div>
            ) : (
              <div className="side-history">
                {submissions.length === 0 ? (
                  <p className="muted-msg" style={{ padding: "var(--s5)" }}>No submissions yet.</p>
                ) : (
                  <MySubList submissions={submissions} />
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
