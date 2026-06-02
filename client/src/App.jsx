import { useState, useEffect, useRef } from "react";

function App() {
  const [serverStatus, setServerStatus] = useState("Checking backend connection...");
  const [statusMessage, setStatusMessage] = useState("System standby");
  const [concurrency, setConcurrency] = useState(500);
  const [target, setTarget] = useState("https://jsonplaceholder.typicode.com/posts/1");
  const [liveStats, setLiveStats] = useState({
    progress: 0,
    success: 0,
    failed: 0,
    p50_latency: 0,
    p90_latency: 0,
    p99_latency: 0,
    tps: 0,
  });

  const [leaderboard, setLeaderboard] = useState([]);
  const [submissions, setSubmissions] = useState([]);
  const ws = useRef(null);
  const [selectedFile, setSelectedFile] = useState(null);
  const [uploadStatus, setUploadStatus] = useState("Awaiting code upload...");
  const [timeRemaining, setTimeRemaining] = useState(0);
  const [contestActive, setContestActive] = useState(false);
  const [questionsData, setQuestionsData] = useState(null);
  const [selectedQuestion, setSelectedQuestion] = useState("q1");

  const formatTime = (seconds) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  useEffect(() => {
    // Health check
    fetch("http://localhost:9000/api/health")
      .then((response) => response.json())
      .then((data) => {
        setServerStatus("Go Server is ONLINE (Port 9000)");
      })
      .catch(() => {
        setServerStatus("Go Server is OFFLINE");
      });

    // Sync contest status on load/refresh
    fetch("http://localhost:9000/api/contest/status")
      .then((response) => response.json())
      .then((data) => {
        if (data.active) {
          setContestActive(true);
          setTimeRemaining(data.time_remaining_seconds);
        }
      })
      .catch(() => {});

    // Fetch Questions Data
    fetch("http://localhost:9000/api/questions")
      .then((response) => response.json())
      .then((data) => setQuestionsData(data))
      .catch(() => {});

    // Connect WebSocket
    ws.current = new WebSocket("ws://localhost:9000/api/stream");
    ws.current.onmessage = (event) => {
      const data = JSON.parse(event.data);

      if (data.type === "live_metrics") {
        setLiveStats(data);
        setStatusMessage(`🔥 BOMBARDING TARGET... ${data.progress.toFixed(1)}%`);
        if (data.progress === 100) {
          setStatusMessage("BOMBARDMENT SUCCESSFUL");
        }
      } else if (data.type === "leaderboard") {
        setLeaderboard(data.data || []);
      } else if (data.type === "submission_queued") {
        setSubmissions((prev) => [data.data, ...prev].slice(0, 30));
      } else if (data.type === "submission_graded") {
        setSubmissions((prev) =>
          prev.map((sub) =>
            sub.id === data.data.id ? { ...sub, ...data.data } : sub
          )
        );
      } else if (data.type === "live_feed_log_clear") {
        setSubmissions([]);
      }
    };

    return () => {
      if (ws.current) ws.current.close();
    };
  }, []);

  useEffect(() => {
    let interval = null;
    if (contestActive && timeRemaining > 0) {
      interval = setInterval(() => {
        setTimeRemaining((prev) => {
          if (prev <= 1) {
            setContestActive(false);
            clearInterval(interval);
            return 0;
          }
          return prev - 1;
        });
      }, 1000);
    } else if (timeRemaining <= 0) {
      setContestActive(false);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [contestActive, timeRemaining]);

  const handleUpload = async () => {
    if (!selectedFile) {
      setUploadStatus("Please select a file first!");
      return;
    }
    setUploadStatus("Uploading to secure sandbox quarantine...");
    const formData = new FormData();
    formData.append("code_file", selectedFile);
    formData.append("team", "You (Contestant)");
    formData.append("question", selectedQuestion);

    try {
      const response = await fetch("http://localhost:9000/api/upload", {
        method: "POST",
        body: formData,
      });
      const data = await response.json();
      if (response.ok) {
        if (data.status === "ignored") {
          setUploadStatus("Ignored: " + data.message);
        } else if (data.submission_id) {
          setUploadStatus(`Uploaded: ID [${data.submission_id.substring(0, 10)}]`);
        } else {
          setUploadStatus("Uploaded successfully.");
        }
      } else {
        setUploadStatus("Sandbox quarantined upload failed.");
      }
    } catch {
      setUploadStatus("Could not establish connection to Backend");
    }
  };

  const launchAttack = async () => {
    setStatusMessage("Triggering distributed attack queue...");
    setLiveStats({ progress: 0, success: 0, failed: 0, p50_latency: 0, p90_latency: 0, p99_latency: 0, tps: 0 });
    try {
      const response = await fetch("http://localhost:9000/api/attack", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          submission_id: "test_run_01",
          target_endpoint: target,
          bot_concurrency: parseInt(concurrency),
          test_duration_seconds: 300,
        }),
      });
      if (response.ok) {
        setStatusMessage("Attack command acknowledged by Master.");
      } else {
        setStatusMessage("Server rejected attack configuration.");
      }
    } catch {
      setStatusMessage("Failed to reach attack pipeline API.");
    }
  };

  const handleStartContest = async () => {
    setStatusMessage("Contest initiating... Preparing sandboxes & launching bot noise!");
    setLeaderboard([]);
    setSubmissions([]);
    try {
      const response = await fetch(`http://localhost:9000/api/start?target=${encodeURIComponent(target)}`, {
        method: "POST",
      });
      const data = await response.json();
      if (response.ok) {
        setStatusMessage("Simulation successfully initialized!");
        setContestActive(true);
        setTimeRemaining(data.time_remaining_seconds || 300);
      } else {
        setStatusMessage("Contest initialization failed.");
      }
    } catch {
      setStatusMessage("Failed to reach contest controller API.");
    }
  };

  const handleReset = async () => {
    if (!window.confirm("Are you sure you want to clear Redis cache and wipe the contest standings?")) return;
    try {
      const response = await fetch("http://localhost:9000/api/reset", { method: "POST" });
      if (response.ok) {
        setLeaderboard([]);
        setSubmissions([]);
        setContestActive(false);
        setTimeRemaining(0);
        alert("Contest scoreboard has been completely reset!");
      }
    } catch {
      alert("Failed to connect to reset API.");
    }
  };

  return (
    <div style={styles.appContainer}>
      {/* HEADER SECTION */}
      <header style={styles.header}>
        <div style={styles.headerTitle}>
          <h1 style={styles.brandTitle}>IICPC Summer Hackathon 2026</h1>
          <span style={styles.badge}>DISTRIBUTED BENCHMARKING ENGINE</span>
        </div>
        {contestActive && (
          <div style={styles.countdownBadge}>
            <span style={styles.countdownLabel}>⏳ CONTEST TIME LEFT</span>
            <span style={{ ...styles.countdownValue, color: timeRemaining < 60 ? "#ff1744" : "#64ffda" }}>
              {formatTime(timeRemaining)}
            </span>
          </div>
        )}
        <div style={{ ...styles.serverIndicator, borderColor: serverStatus.includes("ONLINE") ? "#00e676" : "#ff1744" }}>
          <span style={{ ...styles.statusDot, background: serverStatus.includes("ONLINE") ? "#00e676" : "#ff1744" }}></span>
          {serverStatus}
        </div>
      </header>

      {/* DASHBOARD LAYOUT */}
      <main style={styles.grid}>
        
        {/* LEFT COLUMN: CONTEST SCOREBOARD (THE LEADERBOARD) */}
        <section style={styles.cardCol}>
          <div style={styles.cardHeader}>
            <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
              <h2 style={styles.cardTitle}>🏆 Live Contest Standing</h2>
              {contestActive && (
                <span style={{ ...styles.cardTimerBadge, color: timeRemaining < 60 ? "#ff1744" : "#00b0ff" }}>
                  ⏳ {formatTime(timeRemaining)}
                </span>
              )}
            </div>
            <div style={{ display: "flex", gap: "10px" }}>
              <button onClick={handleStartContest} style={styles.startButton}>🏁 START CONTEST</button>
              <button onClick={handleReset} style={styles.resetButton}>RESET BOARD</button>
            </div>
          </div>
          <div style={styles.leaderboardScrollContainer}>
            {leaderboard.length === 0 ? (
              <div style={styles.emptyState}>Awaiting submission packets from Contest Director...</div>
            ) : (
              <table style={styles.table}>
                <thead>
                  <tr style={styles.trHeader}>
                    <th style={styles.th}>Rank</th>
                    <th style={styles.th}>Competitor Team</th>
                    <th style={{ ...styles.th, textAlign: "center" }}>Attempts</th>
                    <th style={{ ...styles.th, textAlign: "center" }}>Accepted</th>
                    <th style={{ ...styles.th, textAlign: "center" }}>Efficiency</th>
                    <th style={{ ...styles.th, textAlign: "right" }}>Total Points</th>
                  </tr>
                </thead>
                <tbody>
                  {leaderboard.map((item, index) => (
                    <tr key={item.team} style={styles.tr}>
                      <td style={styles.td}>
                        <span style={index === 0 ? styles.rankFirst : index === 1 ? styles.rankSecond : index === 2 ? styles.rankThird : styles.rankNormal}>
                          {index + 1}
                        </span>
                      </td>
                      <td style={styles.td}>
                        <div style={styles.teamName}>{item.team}</div>
                      </td>
                      <td style={{ ...styles.td, textAlign: "center", color: "#b0bec5" }}>
                        {item.attempts || 0}
                      </td>
                      <td style={{ ...styles.td, textAlign: "center", color: "#00e676" }}>
                        {item.accepted || 0}
                      </td>
                      <td style={{ ...styles.td, textAlign: "center", color: "#ffab00" }}>
                        {item.efficiency || "0%"}
                      </td>
                      <td style={{ ...styles.td, textAlign: "right", fontWeight: "bold", color: "#64ffda" }}>
                        {item.score}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </section>

        {/* RIGHT COLUMN: SANDBOX & BOT CONTROLS */}
        <div style={styles.rightCol}>
          
          {/* SANDBOX QUARANTINE SUBMISSION */}
          <section style={styles.card}>
            <h3 style={styles.cardSubTitle}>📦 1. Code Sandbox Submission</h3>
            <p style={styles.subtext}>Upload your exchange algorithm to quarantine and compile inside Docker container.</p>
            
            <div style={{ marginBottom: "12px" }}>
              <label style={styles.label}>Select Question</label>
              <select 
                value={selectedQuestion} 
                onChange={(e) => setSelectedQuestion(e.target.value)}
                style={{ ...styles.textInput, width: "100%", marginBottom: "12px" }}
              >
                {questionsData && Object.keys(questionsData).sort().map(q => (
                  <option key={q} value={q} style={{background: "#020c1b", color: "#64ffda"}}>{q.toUpperCase()}: {questionsData[q].title}</option>
                ))}
              </select>
            </div>
            
            {questionsData && questionsData[selectedQuestion] && (
              <div style={{ background: "#0a192f", padding: "12px", borderRadius: "6px", marginBottom: "16px", border: "1px solid #1e2d3d" }}>
                <p style={{ margin: "0 0 10px 0", color: "#b0bec5", fontSize: "0.85rem", lineHeight: "1.4" }}>
                  {questionsData[selectedQuestion].description}
                </p>
                <div style={{ display: "flex", gap: "12px", fontSize: "0.8rem" }}>
                  <div style={{ flex: 1 }}>
                    <strong style={{ color: "#64ffda" }}>Input Testcase:</strong>
                    <div style={{ background: "#020c1b", padding: "8px", marginTop: "6px", borderRadius: "4px", fontFamily: "monospace", overflowX: "auto" }}>
                      {questionsData[selectedQuestion].sample_input}
                    </div>
                  </div>
                  <div style={{ flex: 1 }}>
                    <strong style={{ color: "#00b0ff" }}>Expected Output:</strong>
                    <div style={{ background: "#020c1b", padding: "8px", marginTop: "6px", borderRadius: "4px", fontFamily: "monospace", overflowX: "auto" }}>
                      {questionsData[selectedQuestion].sample_output}
                    </div>
                  </div>
                </div>
              </div>
            )}

            <div style={styles.uploadRow}>
              <input
                type="file"
                onChange={(e) => setSelectedFile(e.target.files[0])}
                style={styles.fileInput}
              />
              <button onClick={handleUpload} style={styles.uploadButton}>
                📤 UPLOAD FILE
              </button>
            </div>
            <div style={styles.statusBox}>
              <strong>Quarantine Log:</strong> {uploadStatus}
            </div>
          </section>

          {/* BOT FLEET BOMBARDER */}
          <section style={styles.card}>
            <h3 style={styles.cardSubTitle}>🚀 2. Bot Fleet Load Benchmarker</h3>
            <p style={styles.subtext}>Unleash concurrent client workers on the target address to measure latency & TPS.</p>
            <div style={styles.formRow}>
              <div style={styles.formGroup}>
                <label style={styles.label}>Target Endpoint URL</label>
                <input
                  type="text"
                  value={target}
                  onChange={(e) => setTarget(e.target.value)}
                  style={styles.textInput}
                />
              </div>
              <div style={{ ...styles.formGroup, maxWidth: "160px" }}>
                <label style={styles.label}>Bot Concurrency</label>
                <input
                  type="number"
                  value={concurrency}
                  onChange={(e) => setConcurrency(e.target.value)}
                  style={styles.textInput}
                />
              </div>
            </div>
            <button onClick={launchAttack} style={styles.attackButton}>
              🚀 BOMBARD EXCHANGE
            </button>
          </section>

          {/* REALTIME SYSTEM TELEMETRY */}
          <section style={styles.telemetryCard}>
            <h3 style={{ ...styles.cardSubTitle, color: "#00e676" }}>📊 Bot Telemetry Stream</h3>
            <div style={styles.telemetryHeader}>
              <span style={styles.pulseDot}></span>
              <span>{statusMessage}</span>
            </div>

            {/* Progress Bar */}
            <div style={styles.progressContainer}>
              <div style={{ ...styles.progressBar, width: `${liveStats.progress}%` }}></div>
            </div>

            {/* Metrics Grid */}
            <div style={styles.metricsGrid}>
              <div style={styles.metricItem}>
                <span style={styles.metricLabel}>SUCCESS</span>
                <span style={styles.metricVal}>{liveStats.success}</span>
              </div>
              <div style={styles.metricItem}>
                <span style={styles.metricLabel}>FAILURES</span>
                <span style={{ ...styles.metricVal, color: "#ff1744" }}>{liveStats.failed}</span>
              </div>
              <div style={styles.metricItem}>
                <span style={styles.metricLabel}>TPS (SPEED)</span>
                <span style={{ ...styles.metricVal, color: "#00e5ff" }}>{liveStats.tps ? liveStats.tps.toFixed(1) : 0}</span>
              </div>
              <div style={styles.metricItem}>
                <span style={styles.metricLabel}>P90 LATENCY</span>
                <span style={styles.metricVal}>{liveStats.p90_latency}ms</span>
              </div>
            </div>
          </section>
        </div>
      </main>

      {/* CODEFORCES STYLE SUBMISSION TABLE */}
      <section style={styles.activityFeedCard}>
        <h3 style={styles.cardSubTitle}>💻 Live Submissions Feed</h3>
        <div style={styles.leaderboardScrollContainer}>
          {submissions.length === 0 ? (
            <div style={styles.logEmpty}>No submissions recorded yet. Click "🏁 START CONTEST" to begin.</div>
          ) : (
            <table style={styles.table}>
              <thead>
                <tr style={styles.trHeader}>
                  <th style={styles.th}>ID</th>
                  <th style={styles.th}>Time</th>
                  <th style={styles.th}>Team</th>
                  <th style={styles.th}>Problem</th>
                  <th style={{ ...styles.th, textAlign: "center" }}>Latency</th>
                  <th style={{ ...styles.th, textAlign: "center" }}>Verdict</th>
                </tr>
              </thead>
              <tbody>
                {submissions.map((sub) => (
                  <tr key={sub.id} style={styles.tr}>
                    <td style={{ ...styles.td, fontFamily: "monospace", color: "#b0bec5" }}>
                      {sub.id.substring(4, 12)}
                    </td>
                    <td style={styles.td}>
                      {new Date(sub.submit_time).toLocaleTimeString()}
                    </td>
                    <td style={styles.td}>
                      <span style={{ fontWeight: "bold" }}>{sub.team}</span>
                    </td>
                    <td style={styles.td}>
                      <span style={{ color: "#00b0ff" }}>{sub.question}</span>
                    </td>
                    <td style={{ ...styles.td, textAlign: "center", color: "#ffab00" }}>
                      {sub.status === "Running" ? "..." : `${sub.verdict_time - sub.submit_time}ms`}
                    </td>
                    <td style={{ ...styles.td, textAlign: "center", fontWeight: "bold", color: sub.verdict === "AC" ? "#00e676" : sub.verdict ? "#ff1744" : "#b0bec5" }}>
                      {sub.status === "Running" ? "In Queue / Running" : sub.verdict}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </section>
    </div>
  );
}

// PREMIUM DESIGN SYSTEM CONSTANTS & STYLES (SLICK DARK NEON THEME)
const styles = {
  appContainer: {
    backgroundColor: "#0d0e12",
    backgroundImage: "radial-gradient(circle at 10% 20%, rgba(20, 24, 38, 0.95) 0%, rgba(13, 14, 18, 1) 90.2%)",
    color: "#e2e8f0",
    minHeight: "100vh",
    fontFamily: "'Outfit', 'Inter', system-ui, sans-serif",
    padding: "30px 40px",
  },
  header: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    borderBottom: "1px solid rgba(255, 255, 255, 0.08)",
    paddingBottom: "25px",
    marginBottom: "35px",
  },
  headerTitle: {
    display: "flex",
    flexDirection: "column",
    gap: "5px",
  },
  brandTitle: {
    fontSize: "1.8rem",
    fontWeight: "800",
    background: "linear-gradient(90deg, #64ffda, #00b0ff)",
    WebkitBackgroundClip: "text",
    WebkitTextFillColor: "transparent",
    margin: 0,
    letterSpacing: "-0.05em",
  },
  badge: {
    fontSize: "0.75rem",
    color: "#8a99ad",
    letterSpacing: "0.2em",
    fontWeight: "700",
  },
  serverIndicator: {
    display: "flex",
    alignItems: "center",
    gap: "10px",
    fontSize: "0.85rem",
    color: "#e2e8f0",
    backgroundColor: "rgba(255, 255, 255, 0.03)",
    padding: "8px 16px",
    borderRadius: "20px",
    border: "1px solid rgba(255,255,255,0.05)",
  },
  statusDot: {
    width: "8px",
    height: "8px",
    borderRadius: "50%",
  },
  grid: {
    display: "grid",
    gridTemplateColumns: "1fr 1fr",
    gap: "30px",
    marginBottom: "30px",
  },
  cardCol: {
    backgroundColor: "rgba(18, 20, 29, 0.7)",
    backdropFilter: "blur(12px)",
    border: "1px solid rgba(255, 255, 255, 0.05)",
    borderRadius: "16px",
    padding: "25px",
    display: "flex",
    flexDirection: "column",
  },
  cardHeader: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: "20px",
    borderBottom: "1px solid rgba(255,255,255,0.05)",
    paddingBottom: "15px",
  },
  cardTitle: {
    fontSize: "1.3rem",
    fontWeight: "700",
    color: "#fff",
    margin: 0,
  },
  startButton: {
    backgroundColor: "#00b0ff",
    border: "none",
    color: "#0d0e12",
    padding: "6px 14px",
    borderRadius: "8px",
    fontSize: "0.75rem",
    fontWeight: "bold",
    cursor: "pointer",
    boxShadow: "0 0 10px rgba(0, 176, 255, 0.3)",
    transition: "all 0.2s",
  },
  resetButton: {
    backgroundColor: "transparent",
    border: "1px solid rgba(255, 23, 72, 0.5)",
    color: "#ff1744",
    padding: "6px 12px",
    borderRadius: "8px",
    fontSize: "0.75rem",
    fontWeight: "bold",
    cursor: "pointer",
    transition: "all 0.2s",
  },
  leaderboardScrollContainer: {
    maxHeight: "550px",
    overflowY: "auto",
    flexGrow: 1,
  },
  emptyState: {
    display: "flex",
    justifyContent: "center",
    alignItems: "center",
    height: "200px",
    color: "#5f718a",
    fontSize: "0.95rem",
    fontStyle: "italic",
    textAlign: "center",
  },
  table: {
    width: "100%",
    borderCollapse: "collapse",
  },
  trHeader: {
    borderBottom: "1px solid rgba(255,255,255,0.08)",
  },
  th: {
    padding: "12px 10px",
    color: "#5f718a",
    fontSize: "0.8rem",
    fontWeight: "700",
    textTransform: "uppercase",
    letterSpacing: "0.05em",
    textAlign: "left",
  },
  tr: {
    borderBottom: "1px solid rgba(255, 255, 255, 0.03)",
    transition: "background 0.2s",
  },
  td: {
    padding: "16px 10px",
    fontSize: "0.95rem",
    color: "#e2e8f0",
  },
  rankFirst: { color: "#ffd700", fontWeight: "900", background: "rgba(255, 215, 0, 0.15)", padding: "4px 10px", borderRadius: "6px" },
  rankSecond: { color: "#c0c0c0", fontWeight: "900", background: "rgba(192, 192, 192, 0.15)", padding: "4px 10px", borderRadius: "6px" },
  rankThird: { color: "#cd7f32", fontWeight: "900", background: "rgba(205, 127, 50, 0.15)", padding: "4px 10px", borderRadius: "6px" },
  rankNormal: { color: "#8a99ad", fontWeight: "bold" },
  teamName: {
    fontWeight: "600",
    color: "#fff",
  },
  rightCol: {
    display: "flex",
    flexDirection: "column",
    gap: "30px",
  },
  card: {
    backgroundColor: "rgba(18, 20, 29, 0.6)",
    backdropFilter: "blur(10px)",
    border: "1px solid rgba(255, 255, 255, 0.05)",
    borderRadius: "16px",
    padding: "20px 25px",
  },
  cardSubTitle: {
    fontSize: "1.1rem",
    fontWeight: "700",
    color: "#fff",
    margin: "0 0 5px 0",
  },
  subtext: {
    fontSize: "0.8rem",
    color: "#8a99ad",
    margin: "0 0 15px 0",
  },
  uploadRow: {
    display: "flex",
    gap: "15px",
    alignItems: "center",
    marginBottom: "15px",
  },
  fileInput: {
    backgroundColor: "rgba(255,255,255,0.03)",
    border: "1px solid rgba(255,255,255,0.08)",
    borderRadius: "8px",
    padding: "8px",
    fontSize: "0.85rem",
    color: "#fff",
    flexGrow: 1,
    cursor: "pointer",
  },
  uploadButton: {
    backgroundColor: "#64ffda",
    color: "#0a192f",
    border: "none",
    fontWeight: "700",
    padding: "10px 20px",
    borderRadius: "8px",
    cursor: "pointer",
    transition: "opacity 0.2s",
  },
  statusBox: {
    backgroundColor: "rgba(255,255,255,0.02)",
    border: "1px solid rgba(255,255,255,0.05)",
    borderRadius: "8px",
    padding: "12px",
    fontSize: "0.85rem",
    color: "#8a99ad",
  },
  formRow: {
    display: "flex",
    gap: "15px",
    marginBottom: "15px",
  },
  formGroup: {
    display: "flex",
    flexDirection: "column",
    gap: "5px",
    flexGrow: 1,
  },
  label: {
    fontSize: "0.75rem",
    color: "#8a99ad",
    fontWeight: "700",
    textTransform: "uppercase",
  },
  textInput: {
    backgroundColor: "rgba(0,0,0,0.2)",
    border: "1px solid rgba(255,255,255,0.08)",
    borderRadius: "8px",
    padding: "10px",
    fontSize: "0.9rem",
    color: "#fff",
    outline: "none",
  },
  attackButton: {
    backgroundColor: "#ff1744",
    color: "#fff",
    border: "none",
    fontWeight: "700",
    padding: "12px",
    width: "100%",
    borderRadius: "8px",
    cursor: "pointer",
    fontSize: "0.95rem",
    letterSpacing: "0.05em",
  },
  telemetryCard: {
    backgroundColor: "rgba(18, 20, 29, 0.9)",
    border: "1px solid rgba(0, 230, 118, 0.2)",
    borderRadius: "16px",
    padding: "20px 25px",
  },
  telemetryHeader: {
    display: "flex",
    alignItems: "center",
    gap: "10px",
    fontSize: "0.9rem",
    color: "#00e676",
    fontWeight: "bold",
    margin: "5px 0 15px 0",
  },
  pulseDot: {
    width: "8px",
    height: "8px",
    borderRadius: "50%",
    backgroundColor: "#00e676",
    boxShadow: "0 0 8px #00e676",
    animation: "pulse 1.5s infinite",
  },
  progressContainer: {
    width: "100%",
    backgroundColor: "rgba(255,255,255,0.05)",
    height: "8px",
    borderRadius: "10px",
    overflow: "hidden",
    marginBottom: "20px",
  },
  progressBar: {
    height: "100%",
    backgroundColor: "#00e676",
    boxShadow: "0 0 10px #00e676",
    transition: "width 0.2s",
  },
  metricsGrid: {
    display: "grid",
    gridTemplateColumns: "1fr 1fr",
    gap: "15px",
  },
  metricItem: {
    backgroundColor: "rgba(0,0,0,0.2)",
    border: "1px solid rgba(255,255,255,0.03)",
    padding: "12px 15px",
    borderRadius: "10px",
    display: "flex",
    flexDirection: "column",
    gap: "3px",
  },
  metricLabel: {
    fontSize: "0.7rem",
    color: "#5f718a",
    fontWeight: "700",
  },
  metricVal: {
    fontSize: "1.4rem",
    fontWeight: "800",
    color: "#fff",
  },
  activityFeedCard: {
    backgroundColor: "rgba(18, 20, 29, 0.7)",
    border: "1px solid rgba(255, 255, 255, 0.05)",
    borderRadius: "16px",
    padding: "25px",
  },
  logContainer: {
    backgroundColor: "#07080b",
    border: "1px solid rgba(255,255,255,0.04)",
    borderRadius: "10px",
    padding: "15px 20px",
    height: "200px",
    overflowY: "auto",
    fontFamily: "'Fira Code', 'Courier New', monospace",
    fontSize: "0.85rem",
    color: "#a0aec0",
    display: "flex",
    flexDirection: "column",
    gap: "8px",
  },
  logEmpty: {
    color: "#5f718a",
    fontSize: "0.9rem",
    fontStyle: "italic",
    textAlign: "center",
    marginTop: "80px",
  },
  logLine: {
    lineHeight: "1.4",
  },
  logTime: {
    color: "#5f718a",
    marginRight: "10px",
    fontWeight: "600",
  },
  countdownBadge: {
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    backgroundColor: "rgba(255, 255, 255, 0.03)",
    padding: "8px 20px",
    borderRadius: "12px",
    border: "1px solid rgba(255, 255, 255, 0.08)",
    boxShadow: "inset 0 0 15px rgba(255, 255, 255, 0.02)",
  },
  countdownLabel: {
    fontSize: "0.65rem",
    color: "#8a99ad",
    fontWeight: "bold",
    letterSpacing: "0.1em",
  },
  countdownValue: {
    fontSize: "1.4rem",
    fontWeight: "900",
    fontFamily: "'Fira Code', monospace",
  },
  cardTimerBadge: {
    fontSize: "0.95rem",
    fontWeight: "800",
    backgroundColor: "rgba(255, 255, 255, 0.04)",
    padding: "4px 10px",
    borderRadius: "6px",
    fontFamily: "'Fira Code', monospace",
    border: "1px solid rgba(255, 255, 255, 0.05)",
  },
};

export default App;
