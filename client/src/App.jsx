import { useState, useEffect, useRef } from "react";
function App() {
  const [serverStatus, setServerStatus] = useState("checking....");
  const [statusMessage, setStatusMessage] = useState("Awaiting orders");
  const [concurrency, setConcurrency] = useState(500);
  const [target, setTarget] = useState(
    "https://jsonplaceholder.typicode.com/posts/1",
  );
  const [liveStats, setLiveStats] = useState({
    progress: 0,
    success: 0,
    failed: 0,
    avg_latency: 0,
  });
  const ws = useRef(null);
  const [selectedFile, setSelectedFile] = useState(null);
  const [uploadStatus, setUploadStatus] = useState("Waiting for submission...");
  useEffect(() => {
    fetch("http://localhost:9000/api/health")
      .then((response) => response.json())
      .then((data) => {
        setServerStatus(data.message);
      })
      .catch((error) => {
        setServerStatus("Go Server is offline");
      });
    ws.current = new WebSocket("ws://localhost:9000/api/stream");
    ws.current.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (data.type === "live_metrics") {
        setLiveStats(data);
        setStatusMessage(
          `🔥 BOMBARDING... ${data.progress.toFixed(1)}% Complete`,
        );
        if (data.progress === 100) {
          setStatusMessage("BOMBARDMENT COMPLETE");
        }
      }
    };
    return () => {
      if (ws.current) ws.current.close();
    };
  }, []);
  const handleUpload = async () => {
    if (!selectedFile) {
      setSelectedFile("please select a file first");
      return;
    }
    setUploadStatus("Uploading to secure quarantine...");
    const formData = new FormData();
    formData.append("code_file", selectedFile);
    try {
      const response = await fetch("http://localhost:9000/api/upload", {
        method: "POST",
        body: formData,
      });
      const data = await response.json();
      if (response.ok) {
        setUploadStatus(`Success: ID[${data.submission_id}$]`);
      } else {
        setUploadStatus("Upload Failed!");
      }
    } catch (error) {
      setUploadStatus("Could not reach Backend", error);
    }
  };
  const launchAttack = async () => {
    setStatusMessage("Sending Attack Command");
    setLiveStats({ progress: 0, success: 0, failed: 0, avg_latency: 0 });
    try {
      const response = await fetch("http://localhost:9000/api/attack", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          submission_id: "test_run_01",
          target_endpoint: target,
          bot_concurrency: parseInt(concurrency),
          test_duration_seconds: 10,
        }),
      });
      const data = await response.json();
      if (response.ok) {
        setStatusMessage("Success: " + data.message);
      } else {
        setStatusMessage("Error: Bad Payload");
      }
    } catch (error) {
      setStatusMessage("Attack Failed to send: ", error);
    }
  };
  return (
    <div
      style={{
        padding: "40px",
        maxWidth: "800px",
        fontFamily: "sans-serif",
        margin: "0 auto",
      }}
    >
      <h1>IICPC Benchmark Dashboard</h1>

      <div
        style={{
          padding: "10px",
          background: "#f0f0f0",
          borderLeft: "5px solid green",
          marginBottom: "20px",
        }}
      >
        <strong>Backend Connection:</strong> {serverStatus}
      </div>
      <div
        style={{
          padding: "20px",
          border: "1px solid #00f",
          marginBottom: "20px",
          borderRadius: "8px",
          background: "#eef",
        }}
      >
        <h3 style={{ margin: "0 0 10px 0", color: "#00a" }}>
          1. Code Submission (Quarantine)
        </h3>

        <input
          type="file"
          onChange={(e) => setSelectedFile(e.target.files[0])}
          style={{ marginBottom: "15px" }}
        />

        <button
          onClick={handleUpload}
          style={{
            padding: "10px 20px",
            background: "#0055ff",
            color: "white",
            fontWeight: "bold",
            border: "none",
            cursor: "pointer",
            borderRadius: "4px",
          }}
        >
          📤 SECURE UPLOAD
        </button>

        <p
          style={{ marginTop: "10px", fontSize: "0.9rem", fontWeight: "bold" }}
        >
          Status: {uploadStatus}
        </p>
      </div>

      <div
        style={{
          padding: "20px",
          border: "1px solid #ccc",
          marginBottom: "20px",
          borderRadius: "8px",
        }}
      >
        <label>Target Endpoint:</label>
        <br />
        <input
          type="text"
          value={target}
          onChange={(e) => setTarget(e.target.value)}
          style={{ width: "95%", marginBottom: "15px", padding: "8px" }}
        />

        <label>Bot Concurrency (Max Concurrent Requests):</label>
        <br />
        <input
          type="number"
          value={concurrency}
          onChange={(e) => setConcurrency(e.target.value)}
          style={{ width: "95%", marginBottom: "15px", padding: "8px" }}
        />

        <button
          onClick={launchAttack}
          style={{
            padding: "12px 24px",
            background: "#d32f2f",
            color: "white",
            fontWeight: "bold",
            border: "none",
            cursor: "pointer",
            width: "100%",
            borderRadius: "4px",
          }}
        >
          🚀 LAUNCH BOT FLEET
        </button>
      </div>

      {/* LIVE TELEMETRY DASHBOARD */}
      <div
        style={{
          background: "#1e1e1e",
          color: "#00ff00",
          padding: "20px",
          borderRadius: "8px",
          fontFamily: "monospace",
        }}
      >
        <h3
          style={{
            margin: "0 0 15px 0",
            borderBottom: "1px solid #333",
            paddingBottom: "10px",
          }}
        >
          Live Telemetry Stream
        </h3>
        <p style={{ fontSize: "1.2rem", margin: "5px 0" }}>
          Status: {statusMessage}
        </p>

        {/* Progress Bar */}
        <div
          style={{
            width: "100%",
            background: "#333",
            height: "20px",
            borderRadius: "10px",
            margin: "15px 0",
          }}
        >
          <div
            style={{
              width: `${liveStats.progress}%`,
              background: "#00ff00",
              height: "100%",
              borderRadius: "10px",
              transition: "width 0.2s",
            }}
          ></div>
        </div>

        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            marginTop: "20px",
          }}
        >
          <div>
            <p style={{ margin: 0, color: "#888" }}>Successful Hits</p>
            <h2 style={{ margin: 0, color: "#fff" }}>{liveStats.success}</h2>
          </div>
          <div>
            <p style={{ margin: 0, color: "#888" }}>Failed Requests</p>
            <h2 style={{ margin: 0, color: "#ff4444" }}>{liveStats.failed}</h2>
          </div>
          <div>
            <p style={{ margin: 0, color: "#888" }}>Avg Latency</p>
            <h2 style={{ margin: 0, color: "#fff" }}>
              {liveStats.avg_latency} ms
            </h2>
          </div>
        </div>
      </div>
    </div>
  );
}
export default App;
