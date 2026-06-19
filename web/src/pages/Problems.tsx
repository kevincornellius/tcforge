import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { api, Problem } from "../api"

export default function Problems() {
  const [problems, setProblems] = useState<Problem[]>([])
  const [err, setErr] = useState("")

  useEffect(() => {
    api.problems()
      .then(setProblems)
      .catch(e => setErr(e.message))
  }, [])

  if (err) return <p className="error">{err}</p>

  return (
    <div className="page">
      <h2>Problems</h2>
      <table className="table">
        <thead>
          <tr>
            <th>#</th>
            <th>Title</th>
            <th>Time</th>
            <th>Memory</th>
          </tr>
        </thead>
        <tbody>
          {problems.map(p => (
            <tr key={p.slug}>
              <td>{p.position}</td>
              <td>
                <Link to={`/problems/${p.slug}`}>{p.title}</Link>
              </td>
              <td>{p.time_limit}s</td>
              <td>{p.memory_limit}MB</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
