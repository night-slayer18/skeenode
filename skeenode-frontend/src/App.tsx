import { useEffect, useState } from 'react';
import './App.css';

interface Job {
  id: string;
  name: string;
  schedule: string;
  command: string;
  status: string;
  next_run_at: string;
}

interface Execution {
  id: string;
  job_id: string;
  status: string;
  scheduled_at: string;
  started_at?: string;
  completed_at?: string;
  exit_code?: number;
}

function App() {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [executions, setExecutions] = useState<Execution[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        // Assuming API is proxied or CORS enabled
        const jobsRes = await fetch('/api/jobs');
        const execsRes = await fetch('/api/executions');

        if (jobsRes.ok) {
          const jobsData = await jobsRes.json();
          setJobs(jobsData);
        }
        if (execsRes.ok) {
          const execsData = await execsRes.json();
          setExecutions(execsData);
        }
        setError(null);
      } catch (err) {
        console.error("Failed to fetch data:", err);
        // Don't show error immediately to avoid flickering on first load if it's just starting up
        if (loading) setError("Failed to connect to backend");
      } finally {
        setLoading(false);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  }, [loading]);

  const createJob = async () => {
      const name = prompt("Job Name:");
      if (!name) return;
      const command = prompt("Command (e.g., echo hello):", "echo hello");
      const schedule = prompt("Schedule (Cron):", "* * * * *");

      try {
          await fetch('/api/jobs', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                  name,
                  command,
                  schedule,
                  type: 'SHELL',
                  owner_id: 'admin'
              })
          });
          // Ideally trigger a refresh here, but fetchData is now inside useEffect.
          // We can just rely on the interval or reload the page.
          // For now, let's reload to be simple as fetchData is scoped.
          window.location.reload();
      } catch (e) {
          console.error(e);
          alert("Failed to create job");
      }
  };

  return (
    <div className="container">
      <header>
        <h1>Skeenode Dashboard</h1>
        <button onClick={createJob}>+ New Job</button>
      </header>

      {error && <div className="error">{error}</div>}
      {loading && <div>Loading...</div>}

      <div className="grid">
        <div className="section">
          <h2>Jobs</h2>
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Schedule</th>
                <th>Status</th>
                <th>Next Run</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map(job => (
                <tr key={job.id}>
                  <td>{job.name}</td>
                  <td><code>{job.schedule}</code></td>
                  <td>
                    <span className={`status ${job.status.toLowerCase()}`}>{job.status}</span>
                  </td>
                  <td>{job.next_run_at ? new Date(job.next_run_at).toLocaleString() : '-'}</td>
                </tr>
              ))}
              {jobs.length === 0 && !loading && <tr><td colSpan={4}>No jobs found</td></tr>}
            </tbody>
          </table>
        </div>

        <div className="section">
          <h2>Recent Executions</h2>
          <table>
            <thead>
              <tr>
                <th>Job ID</th>
                <th>Status</th>
                <th>Scheduled</th>
                <th>Exit Code</th>
              </tr>
            </thead>
            <tbody>
              {executions.map(exec => (
                <tr key={exec.id}>
                  <td title={exec.job_id}>{exec.job_id.slice(0, 8)}...</td>
                  <td>
                     <span className={`status ${exec.status.toLowerCase()}`}>{exec.status}</span>
                  </td>
                  <td>{new Date(exec.scheduled_at).toLocaleTimeString()}</td>
                  <td>{exec.exit_code !== undefined ? exec.exit_code : '-'}</td>
                </tr>
              ))}
               {executions.length === 0 && !loading && <tr><td colSpan={4}>No executions found</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

export default App
