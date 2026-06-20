const BASE = "/api"

function token() {
  return localStorage.getItem("token") ?? ""
}

function headers() {
  return {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token()}`,
  }
}

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: headers(),
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) throw new Error(await res.text())
  if (res.status === 204) return undefined as T
  const text = await res.text()
  try { return JSON.parse(text) }
  catch { throw new Error(`Server returned non-JSON (${res.status}): ${text.slice(0, 100)}`) }
}

export const api = {
  version: () => fetch("/api/version").then(r => r.ok ? r.json() as Promise<{ version: string }> : Promise.resolve({ version: "" })).catch(() => ({ version: "" })),

  login: (username: string, password: string) =>
    req<{ token: string; username: string; display_name: string; is_admin: boolean }>(
      "POST", "/auth/login", { username, password }
    ),

  me: () => req<{ username: string; display_name: string; is_admin: boolean }>("GET", "/auth/me"),

  logout: () => req("POST", "/auth/logout"),

  contest: () => req<ContestState>("GET", "/contest"),

  problems: () => req<Problem[]>("GET", "/problems"),

  problem: (slug: string, lang?: string) =>
    req<{ problem: Problem; statement: string; available_langs: StatementMeta[] }>(
      "GET", `/problems/${slug}${lang ? `?lang=${lang}` : ""}`
    ),

  problemStatements: (slug: string) => req<StatementMeta[]>("GET", `/problems/${slug}/statements`),

  submissions: () => req<Submission[]>("GET", "/submissions"),

  submit: (problem_slug: string, language: string, code: string) =>
    req<{ id: number }>("POST", "/submissions", { problem_slug, language, code }),

  submission: (id: number) =>
    req<{ submission: Submission; verdicts: Verdict[]; subtask_scores: SubtaskScore[] }>("GET", `/submissions/${id}`),

  scoreboard: () => req<ScoreboardEntry[]>("GET", "/scoreboard"),

  subtasks: (slug: string) =>
    req<{ test_groups: number[][]; points: number[] }>("GET", `/problems/${slug}/subtasks`),

  announcements: () => req<Announcement[]>("GET", "/announcements"),

  admin: {
    // Users
    users: () => req<AdminUser[]>("GET", "/admin/users"),
    createUser: (username: string, password: string, display_name: string, is_admin: boolean) =>
      req<{ id: number }>("POST", "/admin/users", { username, password, display_name, is_admin }),
    deleteUser: (id: number) => req("DELETE", `/admin/users/${id}`),
    resetPassword: (id: number, password: string) =>
      req("PUT", `/admin/users/${id}/password`, { password }),

    // Contest settings
    updateContest: (data: { name: string; duration: string; scoring: string; always_open: boolean; allow_submission: boolean }) =>
      req("PUT", "/admin/contest", data),
    startContest: (startAt?: string) => req("POST", "/admin/contest/start", startAt ? { start_at: new Date(startAt).toISOString() } : {}),
    stopContest: () => req("POST", "/admin/contest/stop"),
    resetContest: () => req("POST", "/admin/contest/reset"),

    // Announcements
    createAnnouncement: (message: string) =>
      req<{ id: number }>("POST", "/admin/announcements", { message }),
    deleteAnnouncement: (id: number) => req("DELETE", `/admin/announcements/${id}`),

    // Submissions
    submissions: () => req<AdminSubmission[]>("GET", "/admin/submissions"),
    rejudge: (id: number) => req("POST", `/admin/submissions/${id}/rejudge`),

    // Problems
    updateProblem: (id: number, data: { title: string; time_limit: number; memory_limit: number }) =>
      req("PUT", `/admin/problems/${id}`, data),
    rebuild: async (id: number, onLine: (line: string) => void): Promise<void> => {
      const res = await fetch(`/api/admin/problems/${id}/rebuild`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token()}` },
      })
      if (!res.ok || !res.body) throw new Error(await res.text())
      const reader = res.body.getReader()
      const dec = new TextDecoder()
      let buf = ""
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        const lines = buf.split("\n")
        buf = lines.pop()!
        for (const line of lines) {
          if (!line.startsWith("data: ")) continue
          const data = line.slice(6)
          if (data === "DONE:ok") return
          if (data === "DONE:error") throw new Error("Build failed — see log above")
          onLine(data)
        }
      }
    },
    uploadStatement: (problemId: number, language: string, file: File): Promise<void> => {
      const form = new FormData()
      form.append("language", language)
      form.append("file", file)
      return fetch(`${BASE}/admin/problems/${problemId}/statements`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token()}` },
        body: form,
      }).then(async res => {
        if (!res.ok) throw new Error(await res.text())
      })
    },
    deleteStatement: (problemId: number, stmtId: number) =>
      req("DELETE", `/admin/problems/${problemId}/statements/${stmtId}`),
  },
}

export interface ContestState {
  name: string
  duration: string
  scoring: string
  always_open: boolean
  allow_submission: boolean
  start_at: string | null
  end_at: string | null
}

export interface StatementMeta {
  language: string
  label: string
  format: string
}

export interface Problem {
  id: number
  slug: string
  title: string
  time_limit: number
  memory_limit: number
  position: number
}

export interface Submission {
  id: number
  problem_slug: string
  problem_title: string
  language: string
  code?: string        // only returned by GET /submissions/:id
  status: string
  verdict: string
  score: number
  time_ms: number
  submitted_at: string
  graded_at: string | null
}

export interface Verdict {
  test_case: string
  verdict: string
  time_ms: number
  memory_kb: number
  group_num: number
}

export interface SubtaskScore {
  subtask_num: number
  verdict: string
  score: number
  max_score: number
}

export interface AdminUser {
  id: number
  username: string
  display_name: string
  is_admin: boolean
}

export interface AdminSubmission {
  id: number
  username: string
  problem_slug: string
  problem_title: string
  language: string
  status: string
  verdict: string
  score: number
  submitted_at: string
  graded_at: string | null
}

export interface Announcement {
  id: number
  message: string
  created_at: string
}

export interface ScoreboardEntry {
  user_id: number
  display_name: string
  total_score: number
  problems: Record<string, number>
}
