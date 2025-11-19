let ws = null;
let statusInterval = null;
let updateTimeout = null;

// initialize
document.addEventListener("DOMContentLoaded", () => {
	loadOptions();
	connectWebSocket();
	setupControls();
	startStatusPolling();
});

// load dropdown options
async function loadOptions() {
	try {
		const [palettes, patterns, colorModes] = await Promise.all([
			fetch("/api/palettes").then((r) => r.json()),
			fetch("/api/patterns").then((r) => r.json()),
			fetch("/api/colorModes").then((r) => r.json()),
		]);

		populateSelect("palette", palettes);
		populateSelect("pattern", patterns);
		populateSelect("colorMode", colorModes);
	} catch (err) {
		console.error("failed to load options:", err);
	}
}

function populateSelect(id, options) {
	const select = document.getElementById(id);
	select.innerHTML = "";
	options.forEach((opt) => {
		const option = document.createElement("option");
		option.value = opt;
		option.textContent = opt;
		select.appendChild(option);
	});
}

// websocket connection
function connectWebSocket() {
	const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
	const wsUrl = `${protocol}//${window.location.host}/ws`;

	ws = new WebSocket(wsUrl);

	ws.onopen = () => {
		updateConnectionStatus("connected");
	};

	ws.onmessage = (event) => {
		try {
			const data = JSON.parse(event.data);
			updateUI(data);
		} catch (err) {
			console.error("failed to parse websocket message:", err);
		}
	};

	ws.onerror = () => {
		updateConnectionStatus("disconnected");
	};

	ws.onclose = () => {
		updateConnectionStatus("disconnected");
		setTimeout(connectWebSocket, 2000);
	};
}

function updateConnectionStatus(status) {
	const indicator = document.getElementById("connection");
	indicator.className = `status-indicator ${status}`;
	indicator.textContent = status;
}

// status polling (fallback)
function startStatusPolling() {
	statusInterval = setInterval(async () => {
		try {
			const response = await fetch("/api/status");
			const data = await response.json();
			updateUI(data);
		} catch (err) {
			console.error("status poll failed:", err);
		}
	}, 500);
}

// update UI with data
function updateUI(data) {
	if (data.fps !== undefined) {
		document.getElementById("fps").textContent = `${data.fps.toFixed(1)} fps`;
	}

	if (data.features) {
		document.getElementById("bass").textContent = data.features.Bass.toFixed(2);
		document.getElementById("mid").textContent = data.features.Mid.toFixed(2);
		document.getElementById("treble").textContent =
			data.features.Treble.toFixed(2);
		document.getElementById("beat").textContent =
			data.features.BeatStrength.toFixed(2);
	}

	if (data.renderer) {
		setSelectValue("pattern", data.renderer.pattern);
		setSelectValue("palette", data.renderer.palette);
		setSelectValue("colorMode", data.renderer.colorMode);
		document.getElementById("colorOnAudio").checked =
			data.renderer.colorOnAudio;
	}

	if (data.params) {
		updateParam("frequency", data.params.Frequency);
		updateParam("amplitude", data.params.Amplitude);
		updateParam("speed", data.params.Speed);
		updateParam("brightness", data.params.Brightness);
		updateParam("contrast", data.params.Contrast);
		updateParam("saturation", data.params.Saturation);
		updateParam("beatSensitivity", data.params.BeatSensitivity);
		updateParam("bassInfluence", data.params.BassInfluence);
		updateParam("midInfluence", data.params.MidInfluence);
		updateParam("trebleInfluence", data.params.TrebleInfluence);
	}
}

function updateParam(id, value) {
	const input = document.getElementById(id);
	if (input && input.value != value) {
		input.value = value;
		const valueSpan = document.getElementById(id + "Value");
		if (valueSpan) {
			valueSpan.textContent = value.toFixed(2);
		}
	}
}

function setSelectValue(id, value) {
	const select = document.getElementById(id);
	if (select && select.value !== value) {
		select.value = value;
	}
}

// setup controls
function setupControls() {
	// pattern, palette, colorMode
	["pattern", "palette", "colorMode"].forEach((id) => {
		document.getElementById(id).addEventListener("change", (e) => {
			sendUpdate({ [id]: e.target.value });
		});
	});

	// colorOnAudio
	document.getElementById("colorOnAudio").addEventListener("change", (e) => {
		sendUpdate({ colorOnAudio: e.target.checked });
	});

	// critical sliders that affect visuals directly - send immediately
	const immediateSliders = [
		"frequency",
		"amplitude",
		"brightness",
		"contrast",
		"saturation",
	];
	immediateSliders.forEach((id) => {
		const input = document.getElementById(id);
		const valueSpan = document.getElementById(id + "Value");
		if (input) {
			input.addEventListener("input", (e) => {
				if (valueSpan) {
					valueSpan.textContent = e.target.value;
				}
				sendUpdate({}); // send immediately
			});
		}
	});

	// other sliders with debounce
	const sliders = [
		"noiseFloor",
		"targetFPS",
		"width",
		"height",
		"speed",
		"beatSensitivity",
		"bassInfluence",
		"midInfluence",
		"trebleInfluence",
		"randomInterval",
		"bufferSize",
	];

	sliders.forEach((id) => {
		const input = document.getElementById(id);
		const valueSpan = document.getElementById(id + "Value");

		if (input) {
			input.addEventListener("input", (e) => {
				if (valueSpan) {
					valueSpan.textContent = e.target.value;
				}
				debouncedUpdate();
			});
		}
	});

	// checkboxes
	document
		.getElementById("autoRandomize")
		.addEventListener("change", debouncedUpdate);

	// buttons
	document.getElementById("randomizeBtn").addEventListener("click", () => {
		// trigger randomize via pattern change
		const patterns = Array.from(document.getElementById("pattern").options).map(
			(o) => o.value
		);
		const randomPattern = patterns[Math.floor(Math.random() * patterns.length)];
		sendUpdate({ pattern: randomPattern });
	});

	// save button
	document.getElementById("saveBtn").addEventListener("click", saveConfig);
}

function saveConfig() {
	const btn = document.getElementById("saveBtn");
	btn.classList.add("saving");
	btn.textContent = "💾 saving...";
	btn.disabled = true;

	// collect all current values
	const config = {
		palette: document.getElementById("palette").value,
		pattern: document.getElementById("pattern").value,
		colorMode: document.getElementById("colorMode").value,
		colorOnAudio: document.getElementById("colorOnAudio").checked,
		noiseFloor: parseFloat(document.getElementById("noiseFloor").value),
		bufferSize: parseInt(document.getElementById("bufferSize").value),
		targetFPS: parseFloat(document.getElementById("targetFPS").value),
		quality: document.getElementById("quality").value,
		width: parseInt(document.getElementById("width").value),
		height: parseInt(document.getElementById("height").value),
		params: {},
	};

	// collect all param values
	const paramIds = [
		"frequency",
		"amplitude",
		"speed",
		"brightness",
		"contrast",
		"saturation",
		"beatSensitivity",
		"bassInfluence",
		"midInfluence",
		"trebleInfluence",
	];

	paramIds.forEach((id) => {
		const input = document.getElementById(id);
		if (input) {
			const value = parseFloat(input.value);
			const paramName = id.charAt(0).toUpperCase() + id.slice(1);
			config.params[paramName] = value;
		}
	});

	fetch("/api/save", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(config),
	})
		.then((r) => r.json())
		.then((data) => {
			btn.classList.remove("saving");
			btn.classList.add("saved");
			btn.textContent = "✓ SAVED";
			setTimeout(() => {
				btn.classList.remove("saved");
				btn.textContent = "💾 SAVE";
				btn.disabled = false;
			}, 2000);
		})
		.catch((err) => {
			console.error("save failed:", err);
			btn.classList.remove("saving");
			btn.textContent = "✗ ERROR";
			setTimeout(() => {
				btn.textContent = "💾 SAVE";
				btn.disabled = false;
			}, 2000);
		});
}

function debouncedUpdate() {
	clearTimeout(updateTimeout);
	updateTimeout = setTimeout(sendUpdate, 100); // reduced to 100ms for real-time feel
}

function sendUpdate(updates) {
	const params = {};

	// collect all param values
	const paramIds = [
		"frequency",
		"amplitude",
		"speed",
		"brightness",
		"contrast",
		"saturation",
		"beatSensitivity",
		"bassInfluence",
		"midInfluence",
		"trebleInfluence",
	];

	paramIds.forEach((id) => {
		const input = document.getElementById(id);
		if (input) {
			const value = parseFloat(input.value);
			const paramName = id.charAt(0).toUpperCase() + id.slice(1);
			params[paramName] = value;
		}
	});

	const payload = {
		...updates,
		params: Object.keys(params).length > 0 ? params : undefined,
	};

	fetch("/api/update", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(payload),
	}).catch((err) => console.error("update failed:", err));
}
