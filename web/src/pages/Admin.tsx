import { useEffect, useState, useRef, FormEvent } from "react"
import { api, AdminUser, AdminSubmission, Announcement, ContestState, Problem, StatementMeta } from "../api"

function fmtDate(iso: string | null | undefined): string {
  if (!iso) return "—"
  return new Date(iso).toLocaleString()
}

type Tab = "users" | "contest" | "problems" | "announcements" | "submissions"

export default function Admin() {
  const [tab, setTab] = useState<Tab>("contest")
  const [version, setVersion] = useState("")

  useEffect(() => {
    api.version().then(v => setVersion(v.version)).catch(() => {})
  }, [])

  return (
    <div className="page admin-page">
      <div className="admin-header">
        <h2>Admin</h2>
        {version && <span className="version-badge">tcforge:{version}</span>}
      </div>
      <div className="admin-tabs">
        {(["contest", "users", "problems", "announcements", "submissions"] as Tab[]).map(t => (
          <button key={t} className={`admin-tab${tab === t ? " active" : ""}`} onClick={() => setTab(t)}>
            {t.charAt(0).toUpperCase() + t.slice(1)}
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
    <div className="admin-section">
      {err
        ? <><p className="error">{err}</p><button onClick={load}>Retry</button></>
        : <p>Loading…</p>
      }
    </div>
  )

  const now = new Date()
  const started = state.start_at ? new Date(state.start_at) <= now : false
  const ended = state.end_at ? new Date(state.end_at) <= now : false
  const running = started && !ended

  return (
    <div className="admin-section">
      {err && <p className="error">{err}</p>}

      <div className="contest-status-bar">
        <span className={`contest-badge ${running ? "running" : ended ? "ended" : "idle"}`}>
          {running ? "Running" : ended ? "Ended" : "Not started"}
        </span>
        {state.start_at && <span className="contest-time">Start: {new Date(state.start_at).toLocaleString()}</span>}
        {state.end_at   && <span className="contest-time">End: {new Date(state.end_at).toLocaleString()}</span>}
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
              className="btn-primary"
              onClick={() => act(() => api.admin.startContest(scheduledStart || undefined))}
            >
              {scheduledStart ? "Schedule start" : "Start now"}
            </button>
          </>
        )}
        {running  && <button className="btn-danger" onClick={() => act(api.admin.stopContest)}>Stop contest</button>}
        {(started || ended) && <button className="btn-ghost" onClick={() => { setScheduledStart(""); act(api.admin.resetContest) }}>Reset timer</button>}
      </div>

      <form onSubmit={onSave} className="admin-form">
        <h3>Settings</h3>
        <label>Contest name
          <input value={name} onChange={e => setName(e.target.value)} placeholder="My Contest" required />
        </label>
        <label>Duration
          <input value={duration} onChange={e => setDuration(e.target.value)} placeholder="3h, 90m, 2h30m" />
          <span className="field-hint">Used when clicking "Start contest". Supports Go duration syntax.</span>
        </label>
        <label>Scoring mode
          <select value={scoring} onChange={e => setScoring(e.target.value)}>
            <option value="ioi">IOI (partial, per-subtask)</option>
            <option value="icpc">ICPC (all-or-nothing, stop on first fail)</option>
          </select>
        </label>
        <label className="admin-checkbox" style={{flexDirection:"row", gap:"0.5rem", alignItems:"center"}}>
          <input type="checkbox" checked={alwaysOpen} onChange={e => setAlwaysOpen(e.target.checked)} />
          <span>
            Always open
            <span className="field-hint" style={{display:"block"}}>
              Skip time enforcement — contestants can view problems at any time.
            </span>
          </span>
        </label>
        {alwaysOpen && (
          <label className="admin-checkbox" style={{flexDirection:"row", gap:"0.5rem", alignItems:"center", marginLeft:"1.25rem"}}>
            <input type="checkbox" checked={allowSubmission} onChange={e => setAllowSubmission(e.target.checked)} />
            <span>
              Allow submissions
              <span className="field-hint" style={{display:"block"}}>
                Uncheck to make the contest read-only (view problems, no submitting).
              </span>
            </span>
          </label>
        )}
        <button type="submit" disabled={saving}>{saving ? "Saving…" : "Save settings"}</button>
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
    <div className="admin-section">
      {err && <p className="error">{err}</p>}
      <table className="table">
        <thead><tr><th>#</th><th>Username</th><th>Display name</th><th>Admin</th><th>Actions</th></tr></thead>
        <tbody>
          {users.map(u => (
            <tr key={u.id}>
              <td>{u.id}</td><td>{u.username}</td><td>{u.display_name}</td>
              <td>{u.is_admin ? "yes" : "—"}</td>
              <td className="admin-actions">
                {resetId === u.id ? (
                  <>
                    <input type="password" placeholder="New password" value={resetPw} onChange={e => setResetPw(e.target.value)} />
                    <button onClick={() => onResetPassword(u.id)}>Save</button>
                    <button className="btn-ghost" onClick={() => setResetId(null)}>Cancel</button>
                  </>
                ) : (
                  <>
                    <button className="btn-ghost" onClick={() => { setResetId(u.id); setResetPw("") }}>Reset pw</button>
                    <button className="btn-danger" onClick={() => onDelete(u.id)}>Delete</button>
                  </>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="admin-add">
        <h3>Add user</h3>
        <form onSubmit={onAdd}>
          <input placeholder="Username" value={username} onChange={e => setUsername(e.target.value)} required />
          <input placeholder="Display name (optional)" value={displayName} onChange={e => setDisplayName(e.target.value)} />
          <input type="password" placeholder="Password" value={password} onChange={e => setPassword(e.target.value)} required />
          <label className="admin-checkbox">
            <input type="checkbox" checked={isAdmin} onChange={e => setIsAdmin(e.target.checked)} /> Admin
          </label>
          <button type="submit" disabled={adding}>{adding ? "Adding…" : "Add user"}</button>
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
    <div className="admin-section">
      {err && <p className="error">{err}</p>}
      {problems.map(p => (
        <ProblemEditor
          key={p.id}
          problem={p}
          open={editing === p.id}
          onToggle={() => setEditing(editing === p.id ? null : p.id)}
          onSaved={load}
        />
      ))}
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
    // refresh local state when opened
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
      <div className="problem-editor-header" onClick={onToggle}>
        <span className="problem-editor-title">{problem.slug} — {problem.title}</span>
        <span className="problem-editor-limits">{problem.time_limit}s / {problem.memory_limit}MB</span>
        <span className="problem-editor-chevron">{open ? "▲" : "▼"}</span>
      </div>
      {open && (
        <div className="problem-editor-body">
          {err && <p className="error">{err}</p>}

          <form onSubmit={onSave} className="admin-form problem-form">
            <h4>Settings</h4>
            <label>Title
              <input value={title} onChange={e => setTitle(e.target.value)} required />
            </label>
            <div className="form-row">
              <label>Time limit (s)
                <input type="number" step="0.5" min="0.5" value={tl} onChange={e => setTl(e.target.value)} />
              </label>
              <label>Memory limit (MB)
                <input type="number" min="16" value={ml} onChange={e => setMl(e.target.value)} />
              </label>
            </div>
            <button type="submit" disabled={saving}>{saving ? "Saving…" : "Save"}</button>
          </form>

          <div className="stmt-section">
            <h4>Statements</h4>
            {statements.length > 0 ? (
              <table className="table" style={{marginBottom:"0.75rem"}}>
                <thead><tr><th>Language</th><th>Format</th><th></th></tr></thead>
                <tbody>
                  {statements.map((s: StatementMeta & { id: number }) => (
                    <tr key={s.language}>
                      <td>{s.label}</td>
                      <td>{s.format.toUpperCase()}</td>
                      <td>
                        {s.id
                          ? <button className="btn-danger" style={{padding:"0.2rem 0.5rem",fontSize:"0.8rem"}} onClick={() => onDeleteStmt(s.id)}>Delete</button>
                          : <span style={{color:"#888",fontSize:"0.8rem"}}>legacy</span>
                        }
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : <p style={{color:"#888",marginBottom:"0.75rem"}}>No uploaded statements (using built-in HTML).</p>}

            <form onSubmit={onUpload} className="stmt-upload-form">
              <select value={lang} onChange={e => setLang(e.target.value)}>
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
              <button type="submit" disabled={uploading || !file}>{uploading ? "Uploading…" : "Upload"}</button>
            </form>
            <p className="field-hint">Accepted formats: HTML, Markdown, PDF, TeX</p>
          </div>

          <div className="rebuild-section">
            <h4>Test Cases</h4>
            <p className="field-hint">Re-runs the builder container: recompiles spec.cpp, regenerates tc/, and regenerates config.json (subtask assignments).</p>
            <button
              className={`btn-rebuild ${rebuildStatus}`}
              onClick={onRebuild}
              disabled={rebuildStatus === "building"}
            >
              {rebuildStatus === "building" ? "Building…" : "Rebuild Test Cases"}
            </button>
            {rebuildLog.length > 0 && (
              <pre ref={logRef} className={`rebuild-log rebuild-log-${rebuildStatus}`}>
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
    <div className="admin-section">
      {err && <p className="error">{err}</p>}
      <form onSubmit={onPost} className="announce-form">
        <textarea
          rows={3}
          placeholder="Write an announcement…"
          value={message}
          onChange={e => setMessage(e.target.value)}
          required
        />
        <button type="submit" disabled={posting}>{posting ? "Posting…" : "Post announcement"}</button>
      </form>
      {announcements.length === 0 && <p style={{color:"#888"}}>No announcements yet.</p>}
      <div className="announce-list">
        {announcements.map(a => (
          <div key={a.id} className="announce-item">
            <span className="announce-time">{new Date(a.created_at).toLocaleString()}</span>
            <p className="announce-msg">{a.message}</p>
            <button className="btn-danger" style={{padding:"0.2rem 0.5rem",fontSize:"0.8rem"}} onClick={() => onDelete(a.id)}>Delete</button>
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
    <div className="admin-section">
      {err && <p className="error">{err}</p>}
      <table className="table">
        <thead>
          <tr><th>#</th><th>User</th><th>Problem</th><th>Lang</th><th>Verdict</th><th>Score</th><th>Submitted</th><th>Graded</th><th></th></tr>
        </thead>
        <tbody>
          {subs.map(s => (
            <tr key={s.id}>
              <td><a href={`/submissions/${s.id}`} target="_blank" rel="noreferrer">{s.id}</a></td>
              <td>{s.username}</td>
              <td>{s.problem_slug}</td>
              <td>{s.language}</td>
              <td className={`verdict verdict-${s.verdict}`}>{s.verdict || s.status}</td>
              <td>{s.score}</td>
              <td className="date-cell">{fmtDate(s.submitted_at)}</td>
              <td className="date-cell">{fmtDate(s.graded_at)}</td>
              <td>
                <button
                  className="btn-ghost"
                  style={{padding:"0.2rem 0.6rem",fontSize:"0.8rem"}}
                  disabled={rejudging === s.id}
                  onClick={() => onRejudge(s.id)}
                >
                  {rejudging === s.id ? "…" : "Rejudge"}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
