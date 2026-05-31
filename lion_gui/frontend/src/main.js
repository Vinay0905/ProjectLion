import './style.css';
import './app.css';

import {
    Connect,
    Disconnect,
    SendMessage,
    SelectAndSendFile,
    DownloadFile,
    GetDownloadsDir,
    IsConnected,
    GetNickname
} from '../wailsjs/go/main/App';

import { EventsOn } from '../wailsjs/runtime/runtime';

// State variables
let myNickname = '';
let isConnected = false;

// DOM Elements
const connectScreen = document.getElementById('connect-screen');
const chatScreen = document.getElementById('chat-screen');

// Connection Panel
const inputAddress = document.getElementById('server-address');
const inputNickname = document.getElementById('nickname');
const btnConnect = document.getElementById('btn-connect');
const connectError = document.getElementById('connect-error');

// Sidebar / Profile
const profileNickname = document.getElementById('profile-nickname');
const profileAddress = document.getElementById('profile-address');
const userAvatar = document.getElementById('user-avatar');
const downloadsPath = document.getElementById('downloads-path');
const btnSidebarSend = document.getElementById('btn-sidebar-send');
const btnDisconnect = document.getElementById('btn-disconnect');

// Chat Workspace
const messagesWindow = document.getElementById('messages-window');
const chatInput = document.getElementById('chat-input');
const btnSendText = document.getElementById('btn-send-text');
const btnSendFile = document.getElementById('btn-send-file');

// Toast
const fileToast = document.getElementById('file-transfer-toast');
const toastMessage = document.getElementById('toast-message');

// Load stored values
if (localStorage.getItem('lion_nickname')) {
    inputNickname.value = localStorage.getItem('lion_nickname');
}
if (localStorage.getItem('lion_address')) {
    inputAddress.value = localStorage.getItem('lion_address');
}

// Connect Action
btnConnect.addEventListener('click', async () => {
    const address = inputAddress.value.trim() || 'localhost:9000';
    const nickname = inputNickname.value.trim();

    if (!nickname) {
        showConnectError('Please enter a nickname.');
        return;
    }

    btnConnect.disabled = true;
    showConnectError('Connecting...');

    try {
        const result = await Connect(address, nickname);
        
        // Save to storage
        localStorage.setItem('lion_nickname', nickname);
        localStorage.setItem('lion_address', address);

        myNickname = nickname;
        isConnected = true;

        // Update UI states
        profileNickname.innerText = nickname;
        profileAddress.innerText = address;
        userAvatar.innerText = nickname.charAt(0).toUpperCase();

        // Get Downloads Path
        const path = await GetDownloadsDir();
        downloadsPath.innerText = path;

        // Transition Screen
        connectScreen.classList.remove('active');
        chatScreen.classList.add('active');

        // Reset inputs and messages
        chatInput.value = '';
        messagesWindow.innerHTML = `
            <div class="welcome-message">
                <div class="welcome-badge">Secure TCP Session</div>
                <h2>Welcome to the Chat Room, ${nickname}!</h2>
                <p>Connected to ${address}. Share files and text securely on the local network.</p>
            </div>
        `;
        
        appendSystemMessage('Connected to server successfully.');
        chatInput.focus();

    } catch (err) {
        showConnectError(err.toString());
    } finally {
        btnConnect.disabled = false;
    }
});

// Disconnect Action
btnDisconnect.addEventListener('click', async () => {
    try {
        await Disconnect();
        isConnected = false;
        chatScreen.classList.remove('active');
        connectScreen.classList.add('active');
        showConnectError('Disconnected from server.');
    } catch (err) {
        console.error(err);
    }
});

// Send Message Action
const triggerSendMessage = async () => {
    const text = chatInput.value.trim();
    if (!text) return;

    chatInput.value = '';

    // Handle local slash command UI shortcuts
    if (text.startsWith('/get ')) {
        const file = text.substring(5).trim();
        if (file) {
            try {
                await DownloadFile(file);
                appendSystemMessage(`Requested download of "${file}"...`);
            } catch (err) {
                appendSystemMessage(`Download error: ${err.message}`);
            }
        }
        return;
    }

    try {
        await SendMessage(text);
        // Since server doesn't echo back, append outgoing bubble locally
        appendChatMessage(myNickname, text, true);
    } catch (err) {
        appendSystemMessage(`Failed to send: ${err.toString()}`);
    }
};

btnSendText.addEventListener('click', triggerSendMessage);
chatInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        triggerSendMessage();
    }
});

// Upload File Actions
const triggerUploadFile = async () => {
    try {
        const result = await SelectAndSendFile();
        if (result) {
            appendSystemMessage(`Selected file "${result}". Starting upload...`);
        }
    } catch (err) {
        appendSystemMessage(`File dialog error: ${err.toString()}`);
    }
};

btnSendFile.addEventListener('click', triggerUploadFile);
btnSidebarSend.addEventListener('click', triggerUploadFile);

// Helper to show errors on Connect Screen
function showConnectError(msg) {
    connectError.innerText = msg;
}

// Append System Pill
function appendSystemMessage(text) {
    const row = document.createElement('div');
    row.className = 'system-row';
    
    row.innerHTML = `
        <div class="system-pill">
            <svg style="width: 14px; height: 14px;" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"/>
                <line x1="12" y1="16" x2="12" y2="12"/>
                <line x1="12" y1="8" x2="12.01" y2="8"/>
            </svg>
            <span>${text}</span>
        </div>
    `;
    
    messagesWindow.appendChild(row);
    scrollToBottom();
}

// Append normal or rich File chat message
function appendChatMessage(sender, text, isOutgoing) {
    // Check if system message represents a file sharing announcement
    // Regex matches e.g. "Alice uploaded a file: report.pdf (12345 bytes). Type '/get report.pdf' to download it."
    const fileUploadRegex = /^(.*) uploaded a file: (.*) \((\d+) bytes\)\. Type '\/get (.*)' to download it\.$/;
    const fileMatch = text.match(fileUploadRegex);

    if (fileMatch) {
        const uploader = fileMatch[1];
        const filename = fileMatch[2];
        const bytes = parseInt(fileMatch[3], 10);
        
        appendFileCard(uploader, filename, bytes);
        return;
    }

    const row = document.createElement('div');
    row.className = `message-row ${isOutgoing ? 'outgoing' : 'incoming'}`;

    row.innerHTML = `
        <span class="message-sender">${isOutgoing ? 'You' : sender}</span>
        <div class="message-bubble">${escapeHTML(text)}</div>
    `;

    messagesWindow.appendChild(row);
    scrollToBottom();
}

// Append rich file download card
function appendFileCard(uploader, filename, bytes) {
    const row = document.createElement('div');
    row.className = 'message-row incoming';

    const kbSize = (bytes / 1024).toFixed(1);
    
    row.innerHTML = `
        <span class="message-sender">${uploader} (Shared a file)</span>
        <div class="file-card">
            <div class="file-icon-box">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                    <polyline points="14 2 14 8 20 8"/>
                    <line x1="16" y1="13" x2="8" y2="13"/>
                    <line x1="16" y1="17" x2="8" y2="17"/>
                    <polyline points="10 9 9 9 8 9"/>
                </svg>
            </div>
            <div class="file-details">
                <div class="file-name" title="${filename}">${escapeHTML(filename)}</div>
                <div class="file-size">${kbSize} KB</div>
            </div>
            <button class="file-action-btn" id="dl-btn-${filename.replace(/[^a-zA-Z0-9]/g, '_')}" title="Download File">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                    <polyline points="7 10 12 15 17 10"/>
                    <line x1="12" y1="15" x2="12" y2="3"/>
                </svg>
            </button>
        </div>
    `;

    messagesWindow.appendChild(row);
    scrollToBottom();

    // Bind action to download button
    const dlBtn = row.querySelector('.file-action-btn');
    dlBtn.addEventListener('click', async () => {
        try {
            dlBtn.disabled = true;
            await DownloadFile(filename);
            appendSystemMessage(`Downloading "${filename}"...`);
        } catch (err) {
            appendSystemMessage(`Download request failed: ${err.toString()}`);
            dlBtn.disabled = false;
        }
    });
}

// Scroll to bottom helper
function scrollToBottom() {
    messagesWindow.scrollTop = messagesWindow.scrollHeight;
}

// Escape HTML helper
function escapeHTML(str) {
    return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

// Toast Helpers
function showToast(msg, duration = 3000) {
    toastMessage.innerText = msg;
    fileToast.classList.add('active');
    
    // Clear previous timeout if exists
    if (window.toastTimeout) {
        clearTimeout(window.toastTimeout);
    }

    if (duration > 0) {
        window.toastTimeout = setTimeout(() => {
            fileToast.classList.remove('active');
        }, duration);
    }
}

function hideToast() {
    fileToast.classList.remove('active');
}

// Listen to backend Wails events
EventsOn('message', (data) => {
    if (data.sender === 'System') {
        // Parse system messages that look like uploads
        const fileUploadRegex = /^(.*) uploaded a file: (.*) \((\d+) bytes\)\. Type '\/get (.*)' to download it\.$/;
        if (data.text.match(fileUploadRegex)) {
            appendChatMessage(data.sender, data.text, false);
        } else {
            appendSystemMessage(data.text);
        }
    } else if (data.sender === 'Server') {
        // Handle server system responses (e.g. file confirmations)
        if (data.text.startsWith('FILE_RECEIVED')) {
            const parts = data.text.split(' ');
            const filename = parts[1] || 'file';
            appendSystemMessage(`Upload complete: Server received "${filename}".`);
        } else {
            appendSystemMessage(data.text);
        }
    } else {
        appendChatMessage(data.sender, data.text, false);
    }
});

EventsOn('app_error', (msg) => {
    appendSystemMessage(`Error: ${msg}`);
});

EventsOn('disconnected', (reason) => {
    isConnected = false;
    chatScreen.classList.remove('active');
    connectScreen.classList.add('active');
    showConnectError(reason || 'Disconnected from server.');
});

EventsOn('upload_status', (data) => {
    if (data.status === 'started') {
        showToast(`Uploading: ${data.name}...`, 0);
    } else if (data.status === 'error') {
        showToast(`Upload failed: ${data.error}`);
        appendSystemMessage(`Upload failed for "${data.name}": ${data.error}`);
    } else if (data.status === 'completed') {
        showToast(`Uploaded successfully!`);
    }
});

EventsOn('download_status', (data) => {
    if (data.status === 'started') {
        showToast(`Downloading: ${data.name}...`, 0);
    } else if (data.status === 'error') {
        showToast(`Download failed: ${data.error}`);
        appendSystemMessage(`Download failed for "${data.name}": ${data.error}`);
    } else if (data.status === 'completed') {
        showToast(`Download complete! Saved to downloads folder.`);
        appendSystemMessage(`Successfully downloaded "${data.name}". Saved to: downloads/${data.name}`);
    }
});
