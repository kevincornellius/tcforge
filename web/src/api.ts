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
  return res.json()
}

export const api = {
  login: (username: string, password: string) =>
    req<{ token: string; username: string; display_name: string; is_admin: boolean }>(
      "POST", "/auth/login", { username, password }
    ),

  me: () => req<{ username: string; display_name: string; is_admin: boolean }>("GET", "/auth/me"),

  logout: () => req("POST", "/auth/logout"),

  problems: () => req<Problem[]>("GET", "/problems"),

  problem: (slug: string) =>
    req<{ problem: Problem; statement: string }>("GET", `/problems/${slug}`),

  submissions: () => req<Submission[]>("GET", "/submissions"),

  submit: (problem_slug: string, language: string, code: string) =>
    req<{ id: number }>("POST", "/submissions", { problem_slug, language, code }),

  submission: (id: number) =>
    req<{ submission: Submission; verdicts: Verdict[] }>("GET", `/submissions/${id}`),

  scoreboard: () => req<ScoreboardEntry[]>("GET", "/scoreboard"),

  admin: {
    users: () => req<AdminUser[]>("GET", "/admin/users"),
    createUser: (username: string, password: string, display_name: string, is_admin: boolean) =>
      req<{ id: number }>("POST", "/admin/users", { username, password, display_name, is_admin }),
    deleteUser: (id: number) => req("DELETE", `/admin/users/${id}`),
    resetPassword: (id: number, password: string) =>
      req("PUT", `/admin/users/${id}/password`, { password }),
  },
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
  status: string
  verdict: string
  score: number
  time_ms: number
  submitted_at: string
}

export interface Verdict {
  test_case: string
  verdict: string
  time_ms: number
  memory_kb: number
}

export interface AdminUser {
  id: number
  username: string
  display_name: string
  is_admin: boolean
}

export interface ScoreboardEntry {
  user_id: number
  display_name: string
  total_score: number
  problems: Record<string, number>
}
