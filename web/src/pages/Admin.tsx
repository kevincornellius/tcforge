import { useEffect, useState, useRef, FormEvent } from "react"
import { Link } from "react-router-dom"
import { api, AdminUser, AdminSubmission, Announcement, ContestState, Problem, StatementMeta } from "../api"

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

type Tab = "contest" | "users" | "problems" | "announcements" | "submissions"

export default function Admin() {
  const [tab, setTab] = useState<Tab>("contest")
  const [version, setVersion] = useState("")

  useEffect(() => {
    api.version().then(v => setVersion(v.version)).catch(() => {})
  }, [])

  const tabs: { key: Tab; label: string }[] = [
    { key: "contest",       label: "Contest" },
    { key: "users",         label: "Users" },
    { key: "problems",      label: "Problems" },
    { key: "announcements", label: "Announcements" },
    { key: "submissions",   label: "Submissions" },
  ]

  return (
    <div>
      <div className="admin-top">
        <h1 className="page-title">Admin</h1>
        {version && <span className="version-badge">tcforge v{version}</span>}
      </div>

      <div className="admin-tabs">
        {tabs.map(t => (
          <button
            key={t.key}
            className={`admin-tab${tab === t.key ? " active" : ""}`}
            onClick={() => setTab(t.key)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "contest"       && <ContestTab />}
      {tab === "users"         && <UsersTab />}
      {tab === "problems"      && <ProblemsTab />}
      {tab === "announcements" && <AnnouncementsTab />}
      {tab === "submissions"   && <SubmissionsTab />}
    </div>
  )
}

// ── Contest Tab ──────────────────────────────────────────────────────────────

function ContestTab() {
  const [state, setState] = useState<ContestState | null>(null)
  const [name, setName] = useState("")
  const [duration, setDuration] = useState("")
  const [scoring, setScoring] = useState("ioi")
  const [alwaysOpen, setAlwaysOpen] = useState(true)
  const [allowSubmission, setAllowSubmission] = useState(true)
  const [scheduledStart, setScheduledStart] = useState("")
  const [err, setErr] = useState("")
  const [saving, setSaving] = useState(false)

  function load() {
    api.contest().then(cs => {
      setState(cs)
      setName(cs.name)
      setDuration(cs.duration)
      setScoring(cs.scoring)
      setAlwaysOpen(cs.always_open)
      setAllowSubmission(cs.allow_submission)
    }).catch(e => setErr(e instanceof Error ? e.message : "Failed to load"))
  }
  useEffect(() => { load() }, [])

  async function onSave(e: FormEvent) {
    e.preventDefault()
    setSaving(true); setErr("")
    try {
      await api.admin.updateContest({ name, duration, scoring, always_open: alwaysOpen, allow_submission: allowSubmission })
      load()
    } catch (e: unknown) { setErr(e instanceof Error ? e.message : "Failed") }
    finally { setSaving(false) }
  }

  async function act(fn: () => Promise<unknown>) {
    try { await fn(); load() }
    catch (e: unknown) { setErr(e instanceof Error ? e.message : "Failed") }
  }

  if (!state) return (
    <div>
      {err
        ? <><p className="error-msg">{err}</p><button className="btn btn-ghost btn-sm" onClick={load}>Retry</button></>
        : <p className="muted-msg">Loading…</p>}
    </div>
  )

  const now = new Date()
  const started = state.start_at ? new Date(state.start_at) <= now : false
  const ended   = state.end_at   ? new Date(state.end_at)   <= now : false
  const running = started && !ended

  return (
    <div>
      {err && <p className="error-msg" style={{ marginBottom: "var(--s4)" }}>{err}</p>}

      <div className="status-bar">
        <span className={`status-badge ${running ? "running" : ended ? "ended" : "idle"}`}>
          {running && <span className="status-dot" />}
          {running ? "Running" : ended ? "Ended" : "Not started"}
        </span>
        {state.start_at && <span className="status-time">Start: {new Date(state.start_at).toLocaleString()}</span>}
        {state.end_at   && <span className="status-time">End: {new Date(state.end_at).toLocaleString()}</span>}
      </div>

      <div className="contest-actions">
        {!running && (
          <>
            <input
              type="datetime-local"
              value={scheduledStart}
              onChange={e => setScheduledStart(e.target.value)}
              className="datetime-input"
            />
            <button
              className="btn btn-primary btn-md"
              onClick={() => act(() => api.admin.startContest(scheduledStart || undefined))}
            >
              {scheduledStart ? "Schedule start" : "Start now"}
            </button>
          </>
        )}
        {running && (
          <button className="btn btn-danger btn-md" onClick={() => act(api.admin.stopContest)}>
            Stop contest
          </button>
        )}
        {(started || ended) && (
          <button className="btn btn-ghost btn-md" onClick={() => { setScheduledStart(""); act(api.admin.resetContest) }}>
            Reset timer
          </button>
        )}
      </div>

      <form onSubmit={onSave} className="admin-form">
        <p className="admin-form-title">Settings</p>
        <label className="form-label">
          Contest name
          <input className="input-field" value={name} onChange={e => setName(e.target.value)} placeholder="My Contest" required />
        </label>
        <label className="form-label">
          Duration
          <input className="input-field" value={duration} onChange={e => setDuration(e.target.value)} placeholder="3h, 90m, 2h30m" />
          <span className="field-hint">Used when clicking "Start now". Supports Go duration syntax.</span>
        </label>
        <label className="form-label">
          Scoring mode
          <select className="select-field" value={scoring} onChange={e => setScoring(e.target.value)}>
            <option value="ioi">IOI (partial, per-subtask)</option>
            <option value="icpc">ICPC (all-or-nothing, stop on first fail)</option>
          </select>
        </label>
        <label className="admin-checkbox">
          <input type="checkbox" checked={alwaysOpen} onChange={e => setAlwaysOpen(e.target.checked)} />
          <span>
            Always open
            <span className="field-hint" style={{ display: "block" }}>
              Skip time enforcement — contestants can view problems at any time.
            </span>
          </span>
        </label>
        {alwaysOpen && (
          <label className="admin-checkbox checkbox-indent">
            <input type="checkbox" checked={allowSubmission} onChange={e => setAllowSubmission(e.target.checked)} />
            <span>
              Allow submissions
              <span className="field-hint" style={{ display: "block" }}>
                Uncheck to make the contest read-only.
              </span>
            </span>
          </label>
        )}
        <div>
          <button className="btn btn-dark btn-md" type="submit" disabled={saving}>
            {saving ? "Saving…" : "Save settings"}
          </button>
        </div>
      </form>
    </div>
  )
}

// ── Users Tab ────────────────────────────────────────────────────────────────

function UsersTab() {
  const [users, setUsers] = useState<AdminUser[]>([])
  const [err, setErr] = useState("")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [isAdmin, setIsAdmin] = useState(false)
  const [adding, setAdding] = useState(false)
  const [resetId, setResetId] = useState<number | null>(null)
  const [resetPw, setResetPw] = useState("")

  function load() { api.admin.users().then(setUsers).catch(e => setErr(e.message)) }
  useEffect(() => { load() }, [])

  async function onAdd(e: FormEvent) {
    e.preventDefault(); setErr(""); setAdding(true)
    try {
      await api.admin.createUser(username, password, displayName || username, isAdmin)
      setUsername(""); setPassword(""); setDisplayName(""); setIsAdmin(false)
      load()
    } catch (e: unknown) { setErr(e instanceof Error ? e.message : "Failed") }
    finally { setAdding(false) }
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
    <div>
      {err && <p className="error-msg" style={{ marginBottom: "var(--s4)" }}>{err}</p>}

      <div className="table-wrap">
        <table className="table">
          <thead>
            <tr><th>#</th><th>Username</th><th>Display name</th><th>Role</th><th>Actions</th></tr>
          </thead>
          <tbody>
            {users.map(u => (
              <tr key={u.id}>
                <td className="mono-label">{u.id}</td>
                <td className="mono-label">{u.username}</td>
                <td>{u.display_name}</td>
                <td>
                  {u.is_admin
                    ? <span className="badge" style={{ background: "var(--color-dark)", color: "#fff" }}>admin</span>
                    : <span style={{ color: "var(--color-faint)", fontSize: "0.8rem" }}>—</span>
                  }
                </td>
                <td>
                  <div className="admin-actions">
                    {resetId === u.id ? (
                      <>
                        <input
                          type="password"
                          placeholder="New password"
                          value={resetPw}
                          onChange={e => setResetPw(e.target.value)}
                          className="inline-input"
                        />
                        <button className="btn btn-dark btn-sm" onClick={() => onResetPassword(u.id)}>Save</button>
                        <button className="btn btn-ghost btn-sm" onClick={() => setResetId(null)}>Cancel</button>
                      </>
                    ) : (
                      <>
                        <button className="btn btn-ghost btn-sm" onClick={() => { setResetId(u.id); setResetPw("") }}>Set password</button>
                        <button className="btn btn-danger btn-sm" onClick={() => onDelete(u.id)}>Delete</button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="form-card">
        <p className="form-card-title">Add user</p>
        <form className="inline-form" onSubmit={onAdd}>
          <input className="input-field" style={{ width: 140 }} placeholder="Username" value={username} onChange={e => setUsername(e.target.value)} required />
          <input className="input-field" style={{ width: 160 }} placeholder="Display name (opt.)" value={displayName} onChange={e => setDisplayName(e.target.value)} />
          <input className="input-field" style={{ width: 140 }} type="password" placeholder="Password" value={password} onChange={e => setPassword(e.target.value)} required />
          <label className="admin-checkbox">
            <input type="checkbox" checked={isAdmin} onChange={e => setIsAdmin(e.target.checked)} />
            <span style={{ fontWeight: 500, fontSize: "0.875rem" }}>Admin</span>
          </label>
          <button className="btn btn-dark btn-md" type="submit" disabled={adding}>{adding ? "Adding…" : "Add user"}</button>
        </form>
      </div>
    </div>
  )
}

// ── Problems Tab ─────────────────────────────────────────────────────────────

function ProblemsTab() {
  const [problems, setProblems] = useState<Problem[]>([])
  const [editing, setEditing] = useState<number | null>(null)
  const [err, setErr] = useState("")

  function load() { api.problems().then(setProblems).catch(e => setErr(e.message)) }
  useEffect(() => { load() }, [])

  return (
    <div>
      {err && <p className="error-msg" style={{ marginBottom: "var(--s4)" }}>{err}</p>}
      {problems.map(p => (
        <ProblemEditor
          key={p.id}
          problem={p}
          open={editing === p.id}
          onToggle={() => setEditing(editing === p.id ? null : p.id)}
          onSaved={load}
        />
      ))}
      {problems.length === 0 && !err && <p className="muted-msg">No problems yet.</p>}
    </div>
  )
}

function ProblemEditor({ problem, open, onToggle, onSaved }: {
  problem: Problem
  open: boolean
  onToggle: () => void
  onSaved: () => void
}) {
  const [title, setTitle] = useState(problem.title)
  const [tl, setTl] = useState(String(problem.time_limit))
  const [ml, setMl] = useState(String(problem.memory_limit))
  const [statements, setStatements] = useState<(StatementMeta & { id: number })[]>([])
  const [lang, setLang] = useState("en")
  const [file, setFile] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState("")
  const fileRef = useRef<HTMLInputElement>(null)
  const [rebuildLog, setRebuildLog] = useState<string[]>([])
  const [rebuildStatus, setRebuildStatus] = useState<"idle" | "building" | "ok" | "error">("idle")
  const logRef = useRef<HTMLPreElement>(null)

  useEffect(() => {
    if (!open) return
    setTitle(problem.title)
    setTl(String(problem.time_limit))
    setMl(String(problem.memory_limit))
    loadStatements()
  }, [open])

  function loadStatements() {
    api.problemStatements(problem.slug).then(s => setStatements(s as (StatementMeta & { id: number })[]))
  }

  async function onSave(e: FormEvent) {
    e.preventDefault(); setSaving(true); setErr("")
    try {
      await api.admin.updateProblem(problem.id, {
        title,
        time_limit: parseFloat(tl) || 1,
        memory_limit: parseInt(ml) || 256,
      })
      onSaved()
    } catch (e: unknown) { setErr(e instanceof Error ? e.message : "Failed") }
    finally { setSaving(false) }
  }

  async function onUpload(e: FormEvent) {
    e.preventDefault()
    if (!file) return
    setUploading(true); setErr("")
    try {
      await api.admin.uploadStatement(problem.id, lang, file)
      setFile(null)
      if (fileRef.current) fileRef.current.value = ""
      loadStatements()
    } catch (e: unknown) { setErr(e instanceof Error ? e.message : "Failed") }
    finally { setUploading(false) }
  }

  async function onDeleteStmt(stmtId: number) {
    await api.admin.deleteStatement(problem.id, stmtId).catch(e => setErr(e.message))
    loadStatements()
  }

  async function onRebuild() {
    setRebuildStatus("building")
    setRebuildLog([])
    try {
      await api.admin.rebuild(problem.id, line => {
        setRebuildLog(prev => {
          const next = [...prev, line]
          setTimeout(() => { logRef.current?.scrollTo(0, logRef.current.scrollHeight) }, 0)
          return next
        })
      })
      setRebuildStatus("ok")
    } catch (e: unknown) {
      setRebuildLog(prev => [...prev, e instanceof Error ? e.message : "Unknown error"])
      setRebuildStatus("error")
    }
  }

  return (
    <div className="problem-editor">
      <div className={`ped-header${open ? " open" : ""}`} onClick={onToggle}>
        <span className="ped-slug">{problem.slug}</span>
        <span className="ped-title">{problem.title}</span>
        <span className="ped-limits">{problem.time_limit}s / {problem.memory_limit}MB</span>
        <span className="ped-chevron">▼</span>
      </div>

      {open && (
        <div className="ped-body">
          {err && <p className="error-msg" style={{ marginBottom: "var(--s4)" }}>{err}</p>}

          <form onSubmit={onSave} className="admin-form" style={{ marginBottom: "var(--s5)" }}>
            <p className="ped-form-title">Settings</p>
            <label className="form-label">
              Title
              <input className="input-field" value={title} onChange={e => setTitle(e.target.value)} required />
            </label>
            <div className="form-row">
              <label className="form-label">
                Time limit (s)
                <input className="input-field" type="number" step="0.5" min="0.5" value={tl} onChange={e => setTl(e.target.value)} />
              </label>
              <label className="form-label">
                Memory limit (MB)
                <input className="input-field" type="number" min="16" value={ml} onChange={e => setMl(e.target.value)} />
              </label>
            </div>
            <div>
              <button className="btn btn-dark btn-sm" type="submit" disabled={saving}>{saving ? "Saving…" : "Save"}</button>
            </div>
          </form>

          <div className="stmt-section">
            <p className="stmt-section-title">Statements</p>
            {statements.length > 0 ? (
              <div className="table-wrap" style={{ marginBottom: "var(--s3)" }}>
                <table className="table">
                  <thead>
                    <tr><th>Language</th><th>Format</th><th></th></tr>
                  </thead>
                  <tbody>
                    {statements.map((s: StatementMeta & { id: number }) => (
                      <tr key={s.language}>
                        <td>{s.label}</td>
                        <td className="mono-label">{s.format.toUpperCase()}</td>
                        <td>
                          {s.id
                            ? <button className="btn btn-danger btn-xs" onClick={() => onDeleteStmt(s.id)}>Delete</button>
                            : <span style={{ color: "var(--color-faint)", fontSize: "0.8rem" }}>legacy</span>
                          }
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <p className="muted-msg" style={{ marginBottom: "var(--s3)" }}>No uploaded statements.</p>
            )}

            <form onSubmit={onUpload} className="stmt-upload-form">
              <select value={lang} onChange={e => setLang(e.target.value)} className="select-field" style={{ width: "auto" }}>
                <option value="en">English</option>
                <option value="id">Bahasa Indonesia</option>
                <option value="ja">日本語</option>
                <option value="zh">中文</option>
              </select>
              <input
                ref={fileRef}
                type="file"
                accept=".html,.md,.pdf,.tex"
                onChange={e => setFile(e.target.files?.[0] ?? null)}
                required
              />
              <button className="btn btn-dark btn-sm" type="submit" disabled={uploading || !file}>
                {uploading ? "Uploading…" : "Upload"}
              </button>
            </form>
            <p className="field-hint" style={{ marginTop: "var(--s2)" }}>Accepted: HTML, Markdown, PDF, TeX</p>
          </div>

          <div className="rebuild-section">
            <p className="ped-form-title">Test Cases</p>
            <p className="field-hint" style={{ marginBottom: "var(--s3)" }}>
              Re-runs the builder container: recompiles spec.cpp, regenerates tc/ and config.json.
            </p>
            <button
              className={`btn-rebuild ${rebuildStatus}`}
              onClick={onRebuild}
              disabled={rebuildStatus === "building"}
            >
              {rebuildStatus === "building" ? "Building…" : "Rebuild Test Cases"}
            </button>
            {rebuildLog.length > 0 && (
              <pre ref={logRef} className={`rebuild-log ${rebuildStatus}`}>
                {rebuildLog.join("\n")}
              </pre>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

// ── Announcements Tab ─────────────────────────────────────────────────────────

function AnnouncementsTab() {
  const [announcements, setAnnouncements] = useState<Announcement[]>([])
  const [message, setMessage] = useState("")
  const [posting, setPosting] = useState(false)
  const [err, setErr] = useState("")

  function load() { api.announcements().then(setAnnouncements).catch(e => setErr(e.message)) }
  useEffect(() => { load() }, [])

  async function onPost(e: FormEvent) {
    e.preventDefault(); setPosting(true); setErr("")
    try {
      await api.admin.createAnnouncement(message)
      setMessage(""); load()
    } catch (e: unknown) { setErr(e instanceof Error ? e.message : "Failed") }
    finally { setPosting(false) }
  }

  async function onDelete(id: number) {
    await api.admin.deleteAnnouncement(id).catch(e => setErr(e.message))
    load()
  }

  return (
    <div>
      {err && <p className="error-msg" style={{ marginBottom: "var(--s4)" }}>{err}</p>}

      <div className="announce-compose">
        <form onSubmit={onPost} style={{ display: "flex", flexDirection: "column", gap: "var(--s3)" }}>
          <textarea
            className="textarea-field"
            rows={3}
            placeholder="Write an announcement…"
            value={message}
            onChange={e => setMessage(e.target.value)}
            required
          />
          <div>
            <button className="btn btn-dark btn-md" type="submit" disabled={posting}>
              {posting ? "Posting…" : "Post announcement"}
            </button>
          </div>
        </form>
      </div>

      {announcements.length === 0 && <p className="muted-msg">No announcements yet.</p>}

      <div className="announce-list">
        {announcements.map(a => (
          <div key={a.id} className="announce-card">
            <span className="announce-time">{new Date(a.created_at).toLocaleString()}</span>
            <p className="announce-msg">{a.message}</p>
            <div className="announce-delete">
              <button className="btn btn-danger btn-xs" onClick={() => onDelete(a.id)}>Delete</button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Submissions Tab ───────────────────────────────────────────────────────────

function SubmissionsTab() {
  const [subs, setSubs] = useState<AdminSubmission[]>([])
  const [err, setErr] = useState("")
  const [rejudging, setRejudging] = useState<number | null>(null)

  function load() { api.admin.submissions().then(setSubs).catch(e => setErr(e.message)) }
  useEffect(() => { load() }, [])

  async function onRejudge(id: number) {
    setRejudging(id)
    try { await api.admin.rejudge(id); load() }
    catch (e: unknown) { setErr(e instanceof Error ? e.message : "Failed") }
    finally { setRejudging(null) }
  }

  return (
    <div>
      {err && <p className="error-msg" style={{ marginBottom: "var(--s4)" }}>{err}</p>}
      <div className="admin-sub-list">
        {subs.length === 0 && (
          <p className="muted-msg" style={{ padding: "2rem", textAlign: "center" }}>No submissions yet.</p>
        )}
        {subs.map(s => (
          <div key={s.id} className="admin-sub-row">
            <Link to={`/submissions/${s.id}`} className="admin-sub-id">#{s.id}</Link>
            <div className="admin-sub-who">
              <span className="admin-sub-user">{s.username}</span>
              <Link to={`/problems/${s.problem_slug}`} className="admin-sub-prob">{s.problem_slug}</Link>
              <span className="admin-sub-lang">{fmtLang(s.language)}</span>
            </div>
            <div className="admin-sub-verdict">
              <span className={`badge badge-${s.verdict || s.status}`}>{s.verdict || s.status}</span>
              {s.score > 0 && <span className="admin-sub-score">{s.score}pts</span>}
            </div>
            <div className="admin-sub-perf">
              <span>{s.time_ms > 0 ? `${s.time_ms}ms` : "—"}</span>
              <span>{s.memory_kb > 0 ? `${s.memory_kb}KB` : "—"}</span>
            </div>
            <div className="admin-sub-dates">
              <span>{fmtDate(s.submitted_at)}</span>
              {s.graded_at && <span className="admin-sub-graded">{fmtDate(s.graded_at)}</span>}
            </div>
            <button
              className="btn btn-ghost btn-xs"
              disabled={rejudging === s.id}
              onClick={() => onRejudge(s.id)}
            >
              {rejudging === s.id ? "…" : "Rejudge"}
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
