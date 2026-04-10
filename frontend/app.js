const API_ROOT = (window.APP_CONFIG && window.APP_CONFIG.API_BASE_URL
  ? window.APP_CONFIG.API_BASE_URL
  : "http://localhost:8080").replace(/\/$/, "");
const API_BASE = `${API_ROOT}/api/auth`;
const RESERVATION_API_BASE = `${API_ROOT}/api/reservations`;
const MEDIA_API_BASE = `${API_ROOT}/api/media`;
const TOKEN_KEY = "conference_app_token";
const USER_KEY = "conference_app_user";

const halls = [
  { id: "grand-auditorium", name: "Grand Auditorium", capacity: 300, amenities: "Stage, 4K projector, audio system" },
  { id: "summit-room", name: "Summit Room", capacity: 120, amenities: "Dual screens, video conference setup" },
  { id: "innovation-lab", name: "Innovation Lab", capacity: 60, amenities: "Smart board, modular seating" },
  { id: "strategy-suite", name: "Strategy Suite", capacity: 24, amenities: "Private meeting layout, whiteboards" },
];

const elements = {
  authCard: document.getElementById("auth-card"),
  appCard: document.getElementById("app-card"),
  loginForm: document.getElementById("login-form"),
  registerForm: document.getElementById("register-form"),
  authMessage: document.getElementById("auth-message"),
  tabLogin: document.getElementById("tab-login"),
  tabRegister: document.getElementById("tab-register"),
  welcomeText: document.getElementById("welcome-text"),
  logoutBtn: document.getElementById("logout-btn"),
  hallList: document.getElementById("hall-list"),
  hallSelect: document.getElementById("reservation-hall"),
  reservationForm: document.getElementById("reservation-form"),
  reservationList: document.getElementById("reservation-list"),
  reservationMessage: document.getElementById("reservation-message"),
  mediaForm: document.getElementById("media-form"),
  mediaFileInput: document.getElementById("media-file"),
  mediaMessage: document.getElementById("media-message"),
  mediaList: document.getElementById("media-list"),
};

function loadUser() {
  const raw = localStorage.getItem(USER_KEY);
  return raw ? JSON.parse(raw) : null;
}

function loadToken() {
  return localStorage.getItem(TOKEN_KEY);
}

async function authenticatedFetch(url, options = {}) {
  const token = loadToken();
  if (!token) {
    throw new Error("Please login first.");
  }

  const headers = {
    ...(options.headers || {}),
    Authorization: `Bearer ${token}`,
  };

  const response = await fetch(url, { ...options, headers });

  if (response.status === 401) {
    logout();
    throw new Error("Session expired. Please login again.");
  }

  return response;
}

async function readApiResponse(response) {
  const contentType = (response.headers.get("content-type") || "").toLowerCase();

  if (contentType.includes("application/json")) {
    try {
      return await response.json();
    } catch {
      return {};
    }
  }

  const text = await response.text();
  return text ? { message: text } : {};
}

function setAuthMessage(message, isError = false) {
  elements.authMessage.textContent = message;
  elements.authMessage.style.color = isError ? "#912727" : "#1d5f2f";
}

function setReservationMessage(message, isError = false) {
  elements.reservationMessage.textContent = message;
  elements.reservationMessage.style.color = isError ? "#912727" : "#1d5f2f";
}

function setMediaMessage(message, isError = false) {
  elements.mediaMessage.textContent = message;
  elements.mediaMessage.style.color = isError ? "#912727" : "#1d5f2f";
}

function switchAuthTab(tab) {
  const loginActive = tab === "login";
  elements.loginForm.classList.toggle("hidden", !loginActive);
  elements.registerForm.classList.toggle("hidden", loginActive);
  elements.tabLogin.classList.toggle("active", loginActive);
  elements.tabRegister.classList.toggle("active", !loginActive);
  setAuthMessage("");
}

function renderHalls() {
  elements.hallList.innerHTML = "";
  elements.hallSelect.innerHTML = "";

  for (const hall of halls) {
    const card = document.createElement("div");
    card.className = "hall-item";
    card.innerHTML = `<h4>${hall.name}</h4><p>Capacity: ${hall.capacity}</p><p>${hall.amenities}</p>`;
    elements.hallList.appendChild(card);

    const option = document.createElement("option");
    option.value = hall.id;
    option.textContent = `${hall.name} (${hall.capacity})`;
    elements.hallSelect.appendChild(option);
  }
}

async function renderReservations() {
  elements.reservationList.innerHTML = "";

  let reservations = [];
  try {
    const response = await authenticatedFetch(RESERVATION_API_BASE);
    const data = await readApiResponse(response);
    if (!response.ok) {
      throw new Error(data.error || "Failed to load reservations");
    }
    reservations = data.reservations || [];
  } catch (error) {
    const li = document.createElement("li");
    li.textContent = error.message;
    elements.reservationList.appendChild(li);
    return;
  }

  if (reservations.length === 0) {
    const li = document.createElement("li");
    li.textContent = "No reservations yet.";
    elements.reservationList.appendChild(li);
    return;
  }

  reservations
    .sort((a, b) => `${a.date} ${a.start}`.localeCompare(`${b.date} ${b.start}`))
    .forEach((reservation) => {
      const hall = halls.find((h) => h.id === reservation.hallId);
      const li = document.createElement("li");
      li.textContent = `${reservation.date} ${reservation.start}-${reservation.end} | ${hall?.name ?? reservation.hallId} | ${reservation.attendees} attendees | ${reservation.purpose}`;
      elements.reservationList.appendChild(li);
    });
}

async function setLoggedInState() {
  const user = loadUser();
  const token = loadToken();
  if (!user || !token) {
    elements.authCard.classList.remove("hidden");
    elements.appCard.classList.add("hidden");
    return;
  }

  elements.authCard.classList.add("hidden");
  elements.appCard.classList.remove("hidden");
  elements.welcomeText.textContent = `Welcome, ${user.fullName}`;
  await renderReservations();
  await renderMediaFiles();
}

function formatBytes(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}

async function renderMediaFiles() {
  elements.mediaList.innerHTML = "";

  let files = [];
  try {
    const response = await authenticatedFetch(MEDIA_API_BASE);
    const data = await readApiResponse(response);
    if (!response.ok) {
      throw new Error(data.error || "Failed to load media files");
    }
    files = data.files || [];
  } catch (error) {
    const li = document.createElement("li");
    li.textContent = error.message;
    elements.mediaList.appendChild(li);
    return;
  }

  if (files.length === 0) {
    const li = document.createElement("li");
    li.textContent = "No media files uploaded yet.";
    elements.mediaList.appendChild(li);
    return;
  }

  for (const item of files) {
    const li = document.createElement("li");
    li.className = "media-item";

    const details = document.createElement("div");
    details.className = "media-item-details";
    details.textContent = `${item.fileName} | ${formatBytes(item.sizeBytes)} | ${new Date(item.createdAt).toLocaleString()}`;

    const downloadButton = document.createElement("button");
    downloadButton.type = "button";
    downloadButton.className = "secondary";
    downloadButton.textContent = "Download";
    downloadButton.addEventListener("click", () => downloadMediaFile(item.id, item.fileName));

    li.appendChild(details);
    li.appendChild(downloadButton);
    elements.mediaList.appendChild(li);
  }
}

async function uploadMedia(event) {
  event.preventDefault();
  setMediaMessage("");

  const file = elements.mediaFileInput.files && elements.mediaFileInput.files[0];
  if (!file) {
    setMediaMessage("Please select a file first.", true);
    return;
  }

  const formData = new FormData();
  formData.append("file", file);

  try {
    const response = await authenticatedFetch(MEDIA_API_BASE, {
      method: "POST",
      body: formData,
    });
    const data = await readApiResponse(response);
    if (!response.ok) {
      throw new Error(data.error || "Upload failed");
    }

    elements.mediaForm.reset();
    setMediaMessage("File uploaded successfully.");
    await renderMediaFiles();
  } catch (error) {
    setMediaMessage(error.message, true);
  }
}

async function downloadMediaFile(fileId, fileName) {
  try {
    const response = await authenticatedFetch(`${MEDIA_API_BASE}/${fileId}`);
    if (!response.ok) {
      const data = await readApiResponse(response);
      throw new Error(data.error || "Download failed");
    }

    const blob = await response.blob();
    const link = document.createElement("a");
    const objectUrl = URL.createObjectURL(blob);
    link.href = objectUrl;
    link.download = fileName;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(objectUrl);
  } catch (error) {
    setMediaMessage(error.message, true);
  }
}

async function registerUser(event) {
  event.preventDefault();
  setAuthMessage("");

  const payload = {
    fullName: document.getElementById("register-name").value.trim(),
    email: document.getElementById("register-email").value.trim(),
    password: document.getElementById("register-password").value,
  };

  try {
    const response = await fetch(`${API_BASE}/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    const data = await readApiResponse(response);
    if (!response.ok) {
      throw new Error(data.error || data.message || "Registration failed");
    }

    setAuthMessage("Registration successful. You can now login.");
    elements.registerForm.reset();
    switchAuthTab("login");
  } catch (error) {
    setAuthMessage(error.message, true);
  }
}

async function loginUser(event) {
  event.preventDefault();
  setAuthMessage("");

  const payload = {
    email: document.getElementById("login-email").value.trim(),
    password: document.getElementById("login-password").value,
  };

  try {
    const response = await fetch(`${API_BASE}/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    const data = await readApiResponse(response);
    if (!response.ok) {
      throw new Error(data.error || data.message || "Login failed");
    }

    localStorage.setItem(TOKEN_KEY, data.token);
    localStorage.setItem(USER_KEY, JSON.stringify(data.user));
    elements.loginForm.reset();
    await setLoggedInState();
  } catch (error) {
    setAuthMessage(error.message, true);
  }
}

async function createReservation(event) {
  event.preventDefault();
  setReservationMessage("");

  if (!loadUser() || !loadToken()) {
    setReservationMessage("Please login first.", true);
    return;
  }

  const payload = {
    hallId: elements.hallSelect.value,
    date: document.getElementById("reservation-date").value,
    start: document.getElementById("reservation-start").value,
    end: document.getElementById("reservation-end").value,
    attendees: Number(document.getElementById("reservation-attendees").value),
    purpose: document.getElementById("reservation-purpose").value.trim(),
  };

  if (!payload.date || !payload.start || !payload.end || !payload.purpose) {
    setReservationMessage("Fill in all reservation fields.", true);
    return;
  }

  if (payload.start >= payload.end) {
    setReservationMessage("End time must be after start time.", true);
    return;
  }

  try {
    const response = await authenticatedFetch(RESERVATION_API_BASE, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    const data = await readApiResponse(response);
    if (!response.ok) {
      throw new Error(data.error || data.message || "Failed to create reservation");
    }

    elements.reservationForm.reset();
    await renderReservations();
    setReservationMessage("Reservation created successfully.");
  } catch (error) {
    setReservationMessage(error.message, true);
  }
}

function logout() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
  setLoggedInState();
  switchAuthTab("login");
}

function bindEvents() {
  elements.tabLogin.addEventListener("click", () => switchAuthTab("login"));
  elements.tabRegister.addEventListener("click", () => switchAuthTab("register"));
  elements.loginForm.addEventListener("submit", loginUser);
  elements.registerForm.addEventListener("submit", registerUser);
  elements.reservationForm.addEventListener("submit", createReservation);
  elements.mediaForm.addEventListener("submit", uploadMedia);
  elements.logoutBtn.addEventListener("click", logout);
}

renderHalls();
bindEvents();
setLoggedInState();
