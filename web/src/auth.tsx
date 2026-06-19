import { createContext, useContext, useState, useEffect, ReactNode } from "react"
import { api } from "./api"

interface AuthUser {
  username: string
  display_name: string
  is_admin: boolean
}

interface AuthCtx {
  user: AuthUser | null
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const Ctx = createContext<AuthCtx>(null!)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)

  useEffect(() => {
    if (localStorage.getItem("token")) {
      api.me().then(setUser).catch(() => {
        localStorage.removeItem("token")
      })
    }
  }, [])

  async function login(username: string, password: string) {
    const res = await api.login(username, password)
    localStorage.setItem("token", res.token)
    setUser({ username: res.username, display_name: res.display_name, is_admin: res.is_admin })
  }

  function logout() {
    api.logout().catch(() => {})
    localStorage.removeItem("token")
    setUser(null)
  }

  return <Ctx.Provider value={{ user, login, logout }}>{children}</Ctx.Provider>
}

export function useAuth() {
  return useContext(Ctx)
}
