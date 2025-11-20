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
	// check if it's a visual selector (pattern, palette, colorMode)
	if (id === "pattern" || id === "palette" || id === "colorMode") {
		const container = document.getElementById(id + "-selector");
		if (!container) return;

		container.innerHTML = "";
		options.forEach((opt) => {
			const btn = document.createElement("button");
			btn.className = "option-btn";
			btn.dataset.value = opt;
			btn.textContent = opt;
			btn.addEventListener("click", () => {
				// remove active from all buttons
				container.querySelectorAll(".option-btn").forEach((b) => {
					b.classList.remove("active");
				});
				// add active to clicked button
				btn.classList.add("active");
				// send update immediately
				sendUpdate({ [id]: opt });
			});
			container.appendChild(btn);
		});
	} else {
		// fallback to select for other dropdowns
		const select = document.getElementById(id);
		if (!select) return;
		select.innerHTML = "";
		options.forEach((opt) => {
			const option = document.createElement("option");
			option.value = opt;
			option.textContent = opt;
			select.appendChild(option);
		});
	}
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
		// also update performance section display
		const fpsDisplay = document.getElementById("fps-display");
		if (fpsDisplay) {
			fpsDisplay.textContent = `${data.fps.toFixed(1)} fps`;
		}
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

	if (data.quality) {
		setSelectValue("quality", data.quality);
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
	// handle visual selectors (pattern, palette, colorMode)
	if (id === "pattern" || id === "palette" || id === "colorMode") {
		const container = document.getElementById(id + "-selector");
		if (container) {
			container.querySelectorAll(".option-btn").forEach((btn) => {
				if (btn.dataset.value === value) {
					btn.classList.add("active");
				} else {
					btn.classList.remove("active");
				}
			});
		}
		return;
	}

	// handle quality selector
	if (id === "quality") {
		const container = document.getElementById("quality-selector");
		if (container) {
			container.querySelectorAll(".option-btn").forEach((btn) => {
				if (btn.dataset.value === value) {
					btn.classList.add("active");
				} else {
					btn.classList.remove("active");
				}
			});
		}
		return;
	}

	// fallback to select
	const select = document.getElementById(id);
	if (select && select.value !== value) {
		select.value = value;
	}
}

// setup controls
function setupControls() {
	// quality selector buttons
	const qualitySelector = document.getElementById("quality-selector");
	if (qualitySelector) {
		qualitySelector.querySelectorAll(".option-btn").forEach((btn) => {
			btn.addEventListener("click", () => {
				qualitySelector.querySelectorAll(".option-btn").forEach((b) => {
					b.classList.remove("active");
				});
				btn.classList.add("active");
				sendUpdate({ quality: btn.dataset.value });
			});
		});
	}

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
		const patternSelector = document.getElementById("pattern-selector");
		if (patternSelector) {
			const patterns = Array.from(
				patternSelector.querySelectorAll(".option-btn")
			).map((b) => b.dataset.value);
			const randomPattern =
				patterns[Math.floor(Math.random() * patterns.length)];
			// update UI
			patternSelector.querySelectorAll(".option-btn").forEach((btn) => {
				if (btn.dataset.value === randomPattern) {
					btn.classList.add("active");
				} else {
					btn.classList.remove("active");
				}
			});
			sendUpdate({ pattern: randomPattern });
		}
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
	const getSelectedValue = (selectorId) => {
		const container = document.getElementById(selectorId);
		if (container) {
			const active = container.querySelector(".option-btn.active");
			if (active) return active.dataset.value;
		}
		return "";
	};

	const config = {
		palette: getSelectedValue("palette-selector"),
		pattern: getSelectedValue("pattern-selector"),
		colorMode: getSelectedValue("colorMode-selector"),
		colorOnAudio: document.getElementById("colorOnAudio").checked,
		noiseFloor: parseFloat(document.getElementById("noiseFloor").value),
		bufferSize: parseInt(document.getElementById("bufferSize").value),
		quality: getSelectedValue("quality-selector"),
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

	// collect config values if not in updates
	const config = {};
	if (!updates.noiseFloor) {
		const nf = document.getElementById("noiseFloor");
		if (nf) config.noiseFloor = parseFloat(nf.value);
	}
	if (!updates.bufferSize) {
		const bs = document.getElementById("bufferSize");
		if (bs) config.bufferSize = parseInt(bs.value);
	}
	if (!updates.quality) {
		const q = document.getElementById("quality-selector");
		if (q) {
			const active = q.querySelector(".option-btn.active");
			if (active) config.quality = active.dataset.value;
		}
	}
	if (!updates.width) {
		const w = document.getElementById("width");
		if (w) config.width = parseInt(w.value);
	}
	if (!updates.height) {
		const h = document.getElementById("height");
		if (h) config.height = parseInt(h.value);
	}
	if (!updates.autoRandomize) {
		const ar = document.getElementById("autoRandomize");
		if (ar) config.autoRandomize = ar.checked;
	}
	if (!updates.randomInterval) {
		const ri = document.getElementById("randomInterval");
		if (ri) config.randomInterval = parseInt(ri.value);
	}

	const payload = {
		...config,
		...updates,
		params: Object.keys(params).length > 0 ? params : undefined,
	};

	fetch("/api/update", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(payload),
	}).catch((err) => console.error("update failed:", err));
}
