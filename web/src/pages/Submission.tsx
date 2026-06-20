import { useEffect, useState } from "react"
import { useParams } from "react-router-dom"
import { api, Submission as Sub, Verdict, SubtaskScore } from "../api"

interface SubtaskConfig {
  test_groups: number[][]  // [i] = subtask IDs group i+1 belongs to
  points: number[]         // [j] = points for subtask j+1
}

export default function Submission() {
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
        // fetch subtask config once we know the problem slug
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

  if (err) return <p className="error">{err}</p>
  if (!sub) return <p>Loading…</p>

  const pending = sub.status === "queued" || sub.status === "pending" || sub.status === "judging"

  // group_num -> verdicts
  const byGroup = new Map<number, Verdict[]>()
  for (const v of verdicts) {
    if (!byGroup.has(v.group_num)) byGroup.set(v.group_num, [])
    byGroup.get(v.group_num)!.push(v)
  }

  const hasSubtasks = subtaskScores.length > 0

  // Build subtask view: each subtask -> its groups -> verdicts
  // subtask i (1-indexed) contains groups where test_groups[g-1] includes i
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
    <div className="page">
      <h2>Submission #{sub.id}</h2>
      <p className="sub-meta">
        <strong>{sub.problem_title}</strong> &nbsp;·&nbsp;
        {sub.language} &nbsp;·&nbsp;
        <span className={`verdict verdict-${sub.verdict || sub.status}`}>
          {sub.verdict || sub.status}
        </span>
        &nbsp;·&nbsp; <strong>{sub.score} pts</strong>
        {sub.time_ms > 0 && <> &nbsp;·&nbsp; {sub.time_ms}ms</>}
      </p>

      {pending && (
        <p className="judging-status">
          {sub.status === "queued" ? "Queued…" : `Running… (${verdicts.length} test case${verdicts.length !== 1 ? "s" : ""} done)`}
        </p>
      )}

      {hasSubtasks && (
        <div className="subtask-summary">
          <h3>Subtasks</h3>
          <div className="subtask-grid">
            {subtaskScores.map(s => (
              <div key={s.subtask_num} className={`subtask-card verdict-${s.verdict}`}>
                <div className="subtask-num">Subtask {s.subtask_num}</div>
                <div className="subtask-score">{s.score}/{s.max_score}</div>
                <div className={`verdict verdict-${s.verdict}`}>{s.verdict}</div>
              </div>
            ))}
          </div>
        </div>
      )}

      {verdicts.length > 0 && (
        <div className="verdict-section">
          <h3>Test cases</h3>
          {hasSubtasks && subtaskCfg ? (
            // Group by subtask, then show groups inside each subtask
            subtaskScores.map(s => {
              const groups = subtaskGroups(s.subtask_num)
              return (
                <details key={s.subtask_num} open={s.verdict !== "AC"} className="subtask-details">
                  <summary className="group-summary">
                    Subtask {s.subtask_num}
                    <span className="summary-score">{s.score}/{s.max_score} pts</span>
                    <span className={`verdict verdict-${s.verdict}`}>{s.verdict}</span>
                  </summary>
                  {groups.map(g => (
                    <details key={g} open={groupVerdict(g) !== "AC"} className="group-details">
                      <summary className="group-summary group-summary-inner">
                        Group {g}
                        <span className={`verdict verdict-${groupVerdict(g)}`}>{groupVerdict(g)}</span>
                        <span className="tc-count">{byGroup.get(g)?.length ?? 0} TCs</span>
                      </summary>
                      {byGroup.has(g) && <VerdictTable verdicts={byGroup.get(g)!} />}
                    </details>
                  ))}
                </details>
              )
            })
          ) : hasSubtasks ? (
            // Has subtask scores but no config — group by group_num
            Array.from(byGroup.entries()).sort(([a], [b]) => a - b).map(([g, vs]) => (
              <details key={g} open={vs.some(v => v.verdict !== "AC")}>
                <summary className="group-summary">
                  Group {g}
                  <span className={`verdict verdict-${groupVerdict(g)}`}>{groupVerdict(g)}</span>
                </summary>
                <VerdictTable verdicts={vs} />
              </details>
            ))
          ) : (
            <VerdictTable verdicts={verdicts} />
          )}
        </div>
      )}
    </div>
  )
}

function VerdictTable({ verdicts }: { verdicts: Verdict[] }) {
  return (
    <table className="table verdict-table">
      <thead>
        <tr><th>Test case</th><th>Verdict</th><th>Time</th><th>Memory</th></tr>
      </thead>
      <tbody>
        {verdicts.map(v => (
          <tr key={v.test_case}>
            <td>{v.test_case}</td>
            <td className={`verdict verdict-${v.verdict}`}>{v.verdict}</td>
            <td>{v.time_ms > 0 ? `${v.time_ms}ms` : "—"}</td>
            <td>{v.memory_kb > 0 ? `${v.memory_kb}KB` : "—"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
