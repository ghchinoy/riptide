package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

var (
	mu       sync.Mutex
	lastType string
	clicks   = make(map[string]int)
)

func main() {
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/click", handleClick)
	http.HandleFunc("/type", handleType)
	http.HandleFunc("/status", handleStatus)

	fmt.Println("Test Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
        _, _ = fmt.Fprint(w, `
<html>
<head>
        <title>Riptide Test Bench</title>	<style>
		body { font-family: sans-serif; padding: 20px; }
		.section { border: 1px solid #ccc; padding: 15px; margin-bottom: 20px; border-radius: 8px; }
		.spacer { height: 1000px; background: #f9f9f9; border-left: 5px solid #eee; margin: 20px 0; padding: 20px; }
		button { padding: 10px 20px; cursor: pointer; }
		#result { font-weight: bold; color: green; }
	</style>
</head>
<body>
	<h1>Riptide Test Bench</h1>

	<div class="section" id="basic-interaction">
		<h2>Basic Interaction</h2>
		<button id="click-me" onclick="fetch('/click?id=basic').then(() => document.getElementById('result').innerText = 'Clicked!')">Click Me</button>
		<p id="result">Waiting...</p>
		
		<input type="text" id="type-me" placeholder="Type here..." onchange="fetch('/type?val=' + this.value)">
	</div>

	<div class="spacer">
		<p>... Scrolling Space ...</p>
	</div>

	<div class="section" id="scrolled-interaction">
		<h2>Scrolled Interaction</h2>
		<button id="scrolled-click" onclick="fetch('/click?id=scrolled').then(() => this.innerText = 'Scrolled Clicked!')">Click Me Down Here</button>
	</div>

	<div class="section" id="delayed-section">
		<h2>Delayed Dashboard</h2>
		<div id="loading">Loading dashboard...</div>
		<div id="dashboard" style="display:none;">
			<p>Dashboard Loaded!</p>
			<button id="dashboard-btn" onclick="fetch('/click?id=dashboard')">Action</button>
		</div>
		<script>
			setTimeout(() => {
				document.getElementById('loading').style.display = 'none';
				document.getElementById('dashboard').style.display = 'block';
			}, 2000);
		</script>
	</div>

</body>
</html>
`)
}

func handleClick(w http.ResponseWriter, r *http.Request) {
        mu.Lock()
        defer mu.Unlock()
        id := r.URL.Query().Get("id")
        clicks[id]++
        _, _ = fmt.Fprintf(w, "OK")
}

func handleType(w http.ResponseWriter, r *http.Request) {
        mu.Lock()
        defer mu.Unlock()
        lastType = r.URL.Query().Get("val")
        _, _ = fmt.Fprintf(w, "OK")
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
        mu.Lock()
        defer mu.Unlock()
        w.Header().Set("Content-Type", "application/json")
        _, _ = fmt.Fprintf(w, `{"clicks": %v, "lastType": %q}`, clicks, lastType)
}
