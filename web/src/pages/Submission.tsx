import { useEffect, useState } from "react"
import { useParams, Link } from "react-router-dom"
import { api, Submission as Sub, Verdict, SubtaskScore } from "../api"
import hljs from "highlight.js/lib/core"
import cpp from "highlight.js/lib/languages/cpp"
import python from "highlight.js/lib/languages/python"

hljs.registerLanguage("cpp", cpp)
hljs.registerLanguage("python", python)

const HLJS_LANG: Record<string, string> = { cpp17: "cpp", cpp20: "cpp", python3: "python" }

function fmtDate(iso: string | null | undefined): string {
  if (!iso) return "—"
  return new Date(iso).toLocaleString("en-US", {
    month: "short", day: "numeric",
    hour: "2-digit", minute: "2-digit", hour12: false,
  })
}

function fmtLang(lang: string): string {
  const map: Record<string, string> = { cpp17: "C++17", cpp20: "C++20", python3: "Python 3" }
  return map[lang] ?? lang
}

interface SubtaskConfig {
  test_groups: number[][]
  points: number[]
}

function VerdictBadge({ verdict, status, size }: { verdict: string; status?: string; size?: "lg" }) {
  const v = verdict || status || ""
  return <span className={`badge${size === "lg" ? " badge-lg" : ""} badge-${v}`}>{v || "—"}</span>
}

const ICON_COPY = (
  <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
  </svg>
)
const ICON_CHECK = (
  <svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <polyline points="20 6 9 17 4 12"/>
  </svg>
)

function CopyButton({ text, className }: { text: string; className?: string }) {
  const [copied, setCopied] = useState(false)
  function copy() {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }
  return <button className={className ?? "copy-btn"} onClick={copy} title={copied ? "Copied!" : "Copy"}>{copied ? ICON_CHECK : ICON_COPY}</button>
}

function SourceCode({ code, language }: { code: string; language: string }) {
  const lang = HLJS_LANG[language] ?? "plaintext"
  const highlighted = lang !== "plaintext"
    ? hljs.highlight(code, { language: lang }).value
    : code.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
  return (
    <div className="src-wrap">
      <div className="src-toolbar">
        <CopyButton text={code} className="src-copy-btn" />
      </div>
      <pre className="src-code"><code dangerouslySetInnerHTML={{ __html: highlighted }} /></pre>
    </div>
  )
}

function Accordion({
  label,
  right,
  defaultOpen,
  children,
  inner,
}: {
  label: React.ReactNode
  right?: React.ReactNode
  defaultOpen?: boolean
  children: React.ReactNode
  inner?: boolean
}) {
  const [open, setOpen] = useState(!!defaultOpen)
  if (inner) {
    return (
      <div className="col-inner">
        <button className="col-inner-header" onClick={() => setOpen(o => !o)}>
          <span className={`col-chevron${open ? " open" : ""}`}>▶</span>
          {label}
          <div className="col-right">{right}</div>
        </button>
        {open && <div className="col-inner-content">{children}</div>}
      </div>
    )
  }
  return (
    <div className="col-section">
      <button className="col-header" onClick={() => setOpen(o => !o)}>
        <span className={`col-chevron${open ? " open" : ""}`}>▶</span>
        {label}
        <div className="col-right">{right}</div>
      </button>
      {open && <div className="col-content">{children}</div>}
    </div>
  )
}

export default function SubmissionPage() {
  const { id } = useParams<{ id: string }>()
  const [sub, setSub] = useState<Sub | null>(null)
  const [verdicts, setVerdicts] = useState<Verdict[]>([])
  const [subtaskScores, setSubtaskScores] = useState<SubtaskScore[]>([])
  const [subtaskCfg, setSubtaskCfg] = useState<SubtaskConfig | null>(null)
  const [err, setErr] = useState("")

  useEffect(() => {
    if (!id) return
    const numId = parseInt(id, 10)

    const load = () =>
      api.submission(numId).then(r => {
        setSub(r.submission)
        setVerdicts(r.verdicts ?? [])
        setSubtaskScores(r.subtask_scores ?? [])
        api.subtasks(r.submission.problem_slug).then(cfg => {
          if (cfg.test_groups?.length) setSubtaskCfg(cfg)
        }).catch(() => {})
      })

    load().catch(e => setErr(e.message))

    const interval = setInterval(() => {
      api.submission(numId).then(r => {
        setSub(r.submission)
        setVerdicts(r.verdicts ?? [])
        setSubtaskScores(r.subtask_scores ?? [])
        if (r.submission.status !== "queued" && r.submission.status !== "pending" && r.submission.status !== "judging") {
          clearInterval(interval)
        }
      }).catch(() => clearInterval(interval))
    }, 1500)
    return () => clearInterval(interval)
  }, [id])

  if (err) return <p className="error-msg">{err}</p>
  if (!sub) return <p className="muted-msg">Loading…</p>

  const pending = sub.status === "queued" || sub.status === "pending" || sub.status === "judging"
  const displayVerdict = sub.verdict || sub.status

  const byGroup = new Map<number, Verdict[]>()
  for (const v of verdicts) {
    if (!byGroup.has(v.group_num)) byGroup.set(v.group_num, [])
    byGroup.get(v.group_num)!.push(v)
  }

  const hasSubtasks = subtaskScores.length > 0

  const subtaskGroups = (subtaskNum: number): number[] => {
    if (!subtaskCfg) return []
    return subtaskCfg.test_groups
      .map((subs, idx) => ({ groupNum: idx + 1, subs }))
      .filter(({ subs }) => subs.includes(subtaskNum))
      .map(({ groupNum }) => groupNum)
  }

  const groupVerdict = (groupNum: number): string => {
    const vs = byGroup.get(groupNum) ?? []
    if (vs.length === 0) return "—"
    const fail = vs.find(v => v.verdict !== "AC")
    return fail ? fail.verdict : "AC"
  }

  return (
    <div>
      {/* Header */}
      <div className="sub-card">
        <div className={`sub-verdict-stripe verdict-${displayVerdict}`} />
        <div className="sub-card-inner">
          <div className="sub-info">
            <span className="sub-num">Submission #{sub.id}</span>
            <Link to={`/problems/${sub.problem_slug}`} className="sub-prob-name">
              {sub.problem_title}
            </Link>
            <div className="sub-details">
              <span className="sub-detail-item">
                <span className="sub-detail-label">Lang</span>
                <span className="sub-detail-val">{fmtLang(sub.language)}</span>
              </span>
              {sub.time_ms > 0 && (
                <span className="sub-detail-item">
                  <span className="sub-detail-label">Time</span>
                  <span className="sub-detail-val">{sub.time_ms}ms</span>
                </span>
              )}
              {sub.memory_kb > 0 && (
                <span className="sub-detail-item">
                  <span className="sub-detail-label">Mem</span>
                  <span className="sub-detail-val">{sub.memory_kb}KB</span>
                </span>
              )}
              <span className="sub-detail-item">
                <span className="sub-detail-label">Submitted</span>
                <span className="sub-detail-val">{fmtDate(sub.submitted_at)}</span>
              </span>
              {sub.graded_at && (
                <span className="sub-detail-item">
                  <span className="sub-detail-label">Graded</span>
                  <span className="sub-detail-val">{fmtDate(sub.graded_at)}</span>
                </span>
              )}
            </div>
          </div>
          <div className="sub-result">
            <VerdictBadge verdict={sub.verdict} status={sub.status} size="lg" />
            {sub.score > 0 && <span className="sub-score">{sub.score} pts</span>}
          </div>
        </div>
      </div>

      {/* Compilation error output */}
      {sub.verdict === "CE" && sub.compile_output && (
        <div style={{ marginBottom: "var(--s5)" }}>
          <p className="section-label">Compilation Error</p>
          <pre className="ce-output">{sub.compile_output}</pre>
        </div>
      )}

      {/* Judging progress */}
      {pending && (
        <div className="judging-bar">
          <span className="judging-dot" />
          {sub.status === "queued"
            ? "In queue…"
            : `Judging… (${verdicts.length} test case${verdicts.length !== 1 ? "s" : ""} done)`}
        </div>
      )}

      {/* Subtask summary */}
      {/* Verdict tree */}
      {verdicts.length > 0 && (
        <div style={{ marginBottom: "var(--s5)" }}>
          <p className="section-label">Test Cases</p>
          <div style={{ display: "flex", flexDirection: "column", gap: "var(--s2)" }}>
            {hasSubtasks && subtaskCfg ? (
              subtaskScores.map(s => {
                const groups = subtaskGroups(s.subtask_num)
                const stVerdicts = groups.flatMap(g => byGroup.get(g) ?? [])
                const stMaxTime = stVerdicts.reduce((m, v) => Math.max(m, v.time_ms), 0)
                const stMaxMem  = stVerdicts.reduce((m, v) => Math.max(m, v.memory_kb), 0)
                return (
                  <Accordion
                    key={s.subtask_num}
                    label={`Subtask ${s.subtask_num}`}
                    defaultOpen={s.verdict !== "AC"}
                    right={
                      <>
                        {stMaxTime > 0 && <span className="col-stat">{stMaxTime}ms</span>}
                        {stMaxMem  > 0 && <span className="col-stat">{stMaxMem}KB</span>}
                        <span className="col-score">{s.score}/{s.max_score} pts</span>
                        <VerdictBadge verdict={s.verdict} />
                      </>
                    }
                  >
                    {groups.map(g => {
                      const gVs = byGroup.get(g) ?? []
                      const gMaxTime = gVs.reduce((m, v) => Math.max(m, v.time_ms), 0)
                      const gMaxMem  = gVs.reduce((m, v) => Math.max(m, v.memory_kb), 0)
                      return (
                        <Accordion
                          key={g}
                          inner
                          label={`Group ${g}`}
                          defaultOpen={groupVerdict(g) !== "AC"}
                          right={
                            <>
                              {gMaxTime > 0 && <span className="col-stat">{gMaxTime}ms</span>}
                              {gMaxMem  > 0 && <span className="col-stat">{gMaxMem}KB</span>}
                              <span className="tc-count">{gVs.length} TCs</span>
                              <VerdictBadge verdict={groupVerdict(g)} />
                            </>
                          }
                        >
                          {byGroup.has(g) && <VerdictTable verdicts={byGroup.get(g)!} />}
                        </Accordion>
                      )
                    })}
                    <div style={{ height: "var(--s2)" }} />
                  </Accordion>
                )
              })
            ) : hasSubtasks ? (
              Array.from(byGroup.entries()).sort(([a], [b]) => a - b).map(([g, vs]) => {
                const gMaxTime = vs.reduce((m, v) => Math.max(m, v.time_ms), 0)
                const gMaxMem  = vs.reduce((m, v) => Math.max(m, v.memory_kb), 0)
                return (
                  <Accordion
                    key={g}
                    label={`Group ${g}`}
                    defaultOpen={vs.some(v => v.verdict !== "AC")}
                    right={
                      <>
                        {gMaxTime > 0 && <span className="col-stat">{gMaxTime}ms</span>}
                        {gMaxMem  > 0 && <span className="col-stat">{gMaxMem}KB</span>}
                        <span className="tc-count">{vs.length} TCs</span>
                        <VerdictBadge verdict={groupVerdict(g)} />
                      </>
                    }
                  >
                    <VerdictTable verdicts={vs} />
                  </Accordion>
                )
              })
            ) : (
              <div className="table-wrap">
                <VerdictTable verdicts={verdicts} />
              </div>
            )}
          </div>
        </div>
      )}

      {/* Source code */}
      {sub.code && (
        <details className="src-details">
          <summary className="src-summary">
            Source code
            <span className="tc-count" style={{ marginLeft: "var(--s2)", fontFamily: "var(--mono)" }}>{fmtLang(sub.language)}</span>
          </summary>
          <SourceCode code={sub.code} language={sub.language} />
        </details>
      )}
    </div>
  )
}

function VerdictTable({ verdicts }: { verdicts: Verdict[] }) {
  return (
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
            <td className="mono-label">{v.test_case}</td>
            <td><span className={`badge badge-${v.verdict}`}>{v.verdict}</span></td>
            <td className="mono-label">{v.time_ms > 0 ? `${v.time_ms}ms` : "< 1ms"}</td>
            <td className="mono-label">{v.memory_kb > 0 ? `${v.memory_kb}KB` : "—"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
