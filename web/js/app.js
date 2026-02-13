// --- Token storage encryption (Web Crypto API) ---
const TOKEN_STORAGE_KEYS = { github: 'github_token', gitlab: 'gitlab_token' };
const STORAGE_ENABLED_KEY = 'tokens_storage_enabled';
const PBKDF2_ITERATIONS = 100000;
const AES_GCM_IV_LENGTH = 12;
const SALT_LENGTH = 16;

/**
 * Derives an AES-GCM key from a password string and salt using PBKDF2.
 * @param {string} password - Source string (e.g. origin + app id)
 * @param {Uint8Array} salt - Random salt
 * @returns {Promise<CryptoKey>}
 */
async function deriveKey(password, salt) {
    const enc = new TextEncoder();
    const keyMaterial = await crypto.subtle.importKey(
        'raw',
        enc.encode(password),
        'PBKDF2',
        false,
        ['deriveBits', 'deriveKey']
    );
    return crypto.subtle.deriveKey(
        {
            name: 'PBKDF2',
            salt,
            iterations: PBKDF2_ITERATIONS,
            hash: 'SHA-256',
        },
        keyMaterial,
        { name: 'AES-GCM', length: 256 },
        false,
        ['encrypt', 'decrypt']
    );
}

/**
 * Encrypts a plaintext string for storage. Uses AES-GCM with a key derived
 * from origin + app id and a random salt (stored with the payload).
 * @param {string} plaintext
 * @returns {Promise<string>} Base64-encoded JSON: { s: saltB64, i: ivB64, c: ciphertextB64 }
 */
async function encryptToken(plaintext) {
    const salt = crypto.getRandomValues(new Uint8Array(SALT_LENGTH));
    const iv = crypto.getRandomValues(new Uint8Array(AES_GCM_IV_LENGTH));
    const password = (window.location?.origin || '') + 'kustomap-token-v1';
    const key = await deriveKey(password, salt);

    const enc = new TextEncoder();
    const ciphertext = await crypto.subtle.encrypt(
        { name: 'AES-GCM', iv },
        key,
        enc.encode(plaintext)
    );

    const payload = {
        s: btoa(String.fromCharCode(...salt)),
        i: btoa(String.fromCharCode(...iv)),
        c: btoa(String.fromCharCode(...new Uint8Array(ciphertext))),
    };
    return btoa(unescape(encodeURIComponent(JSON.stringify(payload))));
}

/**
 * Decrypts a payload produced by encryptToken.
 * @param {string} encoded - Base64-encoded JSON payload
 * @returns {Promise<string>} Decrypted plaintext
 */
async function decryptToken(encoded) {
    try {
        const jsonStr = decodeURIComponent(escape(atob(encoded)));
        const payload = JSON.parse(jsonStr);
        const salt = Uint8Array.from(atob(payload.s), (c) => c.charCodeAt(0));
        const iv = Uint8Array.from(atob(payload.i), (c) => c.charCodeAt(0));
        const ciphertext = Uint8Array.from(atob(payload.c), (c) => c.charCodeAt(0));
        const password = (window.location?.origin || '') + 'kustomap-token-v1';
        const key = await deriveKey(password, salt);

        const decrypted = await crypto.subtle.decrypt(
            { name: 'AES-GCM', iv },
            key,
            ciphertext
        );
        return new TextDecoder().decode(decrypted);
    } catch (e) {
        return null;
    }
}

/**
 * Detects if a localStorage value is our encrypted format (v1) or legacy plain text.
 * @param {string} value
 * @returns {boolean}
 */
function isEncryptedPayload(value) {
    if (!value || typeof value !== 'string') return false;
    try {
        const decoded = atob(value);
        const o = JSON.parse(decoded);
        return o && typeof o.s === 'string' && typeof o.i === 'string' && typeof o.c === 'string';
    } catch (_) {
        return false;
    }
}

// renderGraph function with full-window optimization
function renderGraph(container, graphData, onNodeClick) {
    const cy = window.cytoscape({
        container: container,
        elements: graphData.elements,
        style: [
            {
                selector: 'node',
                style: {
                    'label': 'data(label)',
                    'text-valign': 'center',
                    'text-halign': 'center',
                    'width': 280,
                    'height': 44,
                    'font-size': '12px',
                    'text-wrap': 'wrap',
                    'text-max-width': '260px',
                    'border-width': 2,
                    'border-color': '#333',
                    'shape': 'roundrectangle'
                }
            },
            {
                selector: 'node[type="base"]',
                style: {
                    'background-color': '#2ecc71'
                }
            },
            {
                selector: 'node[type="overlay"]',
                style: {
                    'background-color': '#3498db'
                }
            },
            {
                selector: 'node[type="component"]',
                style: {
                    'background-color': '#9b59b6'
                }
            },
            {
                selector: 'node[type="resource"]',
                style: {
                    'background-color': '#3498db'
                }
            },
            {
                selector: 'node[type="error"]',
                style: {
                    'background-color': '#e74c3c',
                    'border-color': '#c0392b',
                    'border-width': 3,
                    'color': 'white'
                }
            },
            {
                selector: 'edge',
                style: {
                    'width': 2,
                    'line-color': '#95a5a6',
                    'target-arrow-color': '#95a5a6',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier',
                    'arrow-scale': 1.2
                }
            },
            {
                selector: 'node:selected',
                style: {
                    'border-width': 4,
                    'border-color': '#e67e22',
                    'background-color': '#f39c12'
                }
            }
        ],
        layout: {
            name: 'dagre',
            rankDir: 'TB',
            nodeSep: 50,
            rankSep: 100,
            padding: 30
        },
        minZoom: 0.1,
        maxZoom: 3,
        wheelSensitivity: 0.2
    });

    cy.on('tap', 'node', function(evt) {
        const node = evt.target;
        if (onNodeClick) {
            onNodeClick(node.data());
        }
    });

    // Resize handler for full-window responsiveness
    window.addEventListener('resize', () => {
        cy.resize();
    });

    return cy;
}

// Setup build modal close/backdrop (call after renderGraph). Build is triggered from sidebar button.
function setupBuildModal(app) {
    const buildModal = document.getElementById('build-modal');
    const buildModalClose = document.getElementById('build-modal-close');
    const buildModalBackdrop = buildModal?.querySelector('.build-modal-backdrop');
    buildModalClose?.addEventListener('click', () => buildModal?.classList.add('hidden'));
    buildModalBackdrop?.addEventListener('click', () => buildModal?.classList.add('hidden'));
}

// Main App
class App {
    constructor() {
        this.form = document.getElementById('analyze-form');
        this.formSection = document.getElementById('form-section');
        this.graphSection = document.getElementById('graph-section');
        this.graphContainer = document.getElementById('graph-container');
        this.errorDiv = document.getElementById('error');
        this.errorMessage = document.getElementById('error-message');
        this.graphErrorDiv = document.getElementById('graph-error');
        this.graphErrorMessage = document.getElementById('graph-error-message');
        this.caBundleWarningDiv = document.getElementById('ca-bundle-warning');
        this.caBundleWarningMessage = document.getElementById('ca-bundle-warning-message');
        this.analyzeBtn = document.getElementById('analyze-btn');
        this.backBtn = document.getElementById('back-btn');
        this.fitBtn = document.getElementById('fit-btn');
        this.centerBtn = document.getElementById('center-btn');
        this.exportPngBtn = document.getElementById('export-png-btn');
        this.exportSvgBtn = document.getElementById('export-svg-btn');
        this.exportMermaidBtn = document.getElementById('export-mermaid-btn');
        this.downloadCABundleBtn = document.getElementById('download-ca-bundle-btn');
        this.localBranchBadge = document.getElementById('local-branch-badge');
        this.localBranchValue = document.getElementById('local-branch-value');
        this.sidebar = document.getElementById('sidebar');
        this.sidebarContent = document.getElementById('sidebar-content');
        this.closeSidebar = document.getElementById('close-sidebar');

        this.currentGraphId = null;
        this.currentGraphData = null;
        this.cy = null;
        this.localEnabled = false;
        this.browseCurrentPath = '';

        this.init();
    }

    async init() {
        this.form.addEventListener('submit', (e) => this.handleSubmit(e));
        this.backBtn.addEventListener('click', () => this.showForm());
        this.closeSidebar.addEventListener('click', () => this.hideSidebar());
        this.fitBtn.addEventListener('click', () => this.fitGraph());
        this.centerBtn.addEventListener('click', () => this.centerGraph());
        this.exportPngBtn.addEventListener('click', () => this.exportPNG());
        this.exportSvgBtn.addEventListener('click', () => this.exportSVG());
        this.exportMermaidBtn.addEventListener('click', () => this.exportMermaid());
        this.downloadCABundleBtn.addEventListener('click', () => this.downloadCABundle());

        this.loadTokensFromStorage();
        this.setupTokenPersistence();
        this.setupTokenVisibilityToggles();
        document.getElementById('clear-tokens-btn')?.addEventListener('click', () => this.clearStoredTokens());

        try {
            const res = await fetch('/api/v1/config');
            if (res.ok) {
                const cfg = await res.json();
                this.localEnabled = !!cfg.local_enabled;
                this.updateFormForLocalMode();
                this.setupBrowseModal();
            }
        } catch (_) {
            /* config fetch failed; keep defaults */
        }
    }

    updateFormForLocalMode() {
        const urlInputRow = document.getElementById('url-input-row');
        const browseBtn = document.getElementById('browse-btn');
        const urlInput = document.getElementById('url');

        if (this.localEnabled && browseBtn) {
            browseBtn.classList.remove('hidden');
            if (urlInputRow && !urlInputRow.classList.contains('input-with-browse')) {
                urlInputRow.classList.remove('hidden');
            }
            if (urlInput) {
                urlInput.placeholder = 'e.g. https://github.com/user/repo or ~/my-kustomize/repo';
            }
        } else if (browseBtn) {
            browseBtn.classList.add('hidden');
            if (urlInput) {
                urlInput.placeholder = 'e.g., https://github.com/user/repo or gitlab:user/repo';
            }
        }
    }

    setupBrowseModal() {
        const modal = document.getElementById('browse-modal');
        const closeBtn = document.getElementById('browse-modal-close');
        const backdrop = document.getElementById('browse-modal-backdrop');
        const browseBtn = document.getElementById('browse-btn');
        const usePathBtn = document.getElementById('browse-use-path-btn');

        if (!modal || !browseBtn) return;

        browseBtn.addEventListener('click', () => this.openBrowseModal());

        const closeModal = () => modal?.classList.add('hidden');
        closeBtn?.addEventListener('click', closeModal);
        backdrop?.addEventListener('click', closeModal);

        usePathBtn?.addEventListener('click', () => {
            const urlInput = document.getElementById('url');
            if (urlInput && this.browseCurrentPath) {
                urlInput.value = this.browseCurrentPath;
            }
            closeModal();
        });
    }

    async openBrowseModal() {
        const modal = document.getElementById('browse-modal');
        if (!modal) return;
        modal.classList.remove('hidden');
        this.browseCurrentPath = '';
        await this.loadBrowseDirs();
    }

    async loadBrowseDirs(path) {
        const listEl = document.getElementById('browse-list');
        const loadingEl = document.getElementById('browse-loading');
        const breadcrumbEl = document.getElementById('browse-breadcrumb');
        const usePathBtn = document.getElementById('browse-use-path-btn');

        if (!listEl || !loadingEl) return;

        loadingEl.classList.remove('hidden');
        listEl.innerHTML = '';
        if (usePathBtn) usePathBtn.disabled = true;

        /* Always use POST with JSON body to avoid URL length limits and keep one code path */
        const fetchOpts = {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: path || '' })
        };

        try {
            const res = await fetch('/api/v1/browse', fetchOpts);
            if (!res.ok) {
                const err = await res.json().catch(() => ({}));
                throw new Error(err.message || res.statusText);
            }
            let dirs;
            try {
                dirs = await res.json();
            } catch (parseErr) {
                throw new Error('Invalid JSON in response: ' + (parseErr.message || 'parse error'));
            }
            if (!Array.isArray(dirs)) {
                const hint = dirs === null ? 'null' : (typeof dirs === 'object' ? (dirs.message || JSON.stringify(dirs).slice(0, 80)) : String(dirs));
                throw new Error('Server returned unexpected format (expected array): ' + hint);
            }
            this.browseCurrentPath = path || '~';

            /* Build breadcrumb */
            if (breadcrumbEl) {
                const parts = (path || '').split('/').filter(Boolean);
                let html = '<a href="#" data-path="">Home</a>';
                parts.forEach((p, i) => {
                    const acc = '/' + parts.slice(0, i + 1).join('/');
                    html += ' <span class="separator">/</span> <a href="#" data-path="' + acc + '">' + p + '</a>';
                });
                breadcrumbEl.innerHTML = html;
                breadcrumbEl.querySelectorAll('a').forEach(a => {
                    a.addEventListener('click', (e) => {
                        e.preventDefault();
                        this.loadBrowseDirs(a.dataset.path ?? '');
                    });
                });
            }

            if (dirs.length === 0 && path) {
                listEl.innerHTML = '<li style="cursor:default;color:#666;">No subdirectories here. Use "Use this path" to select.</li>';
            }
            dirs.forEach(dirPath => {
                const name = dirPath.split(/[/\\]/).filter(Boolean).pop() || dirPath;
                const li = document.createElement('li');
                li.innerHTML = '<span class="icon-folder">📁</span> ' + name;
                li.dataset.path = dirPath;
                li.addEventListener('click', () => this.loadBrowseDirs(dirPath));
                listEl.appendChild(li);
            });
            if (usePathBtn) {
                usePathBtn.disabled = false;
                usePathBtn.textContent = 'Use this path';
            }
        } catch (err) {
            listEl.innerHTML = '<li style="cursor:default;color:#c33;">' + (err.message || 'Failed to load') + '</li>';
        } finally {
            loadingEl.classList.add('hidden');
        }
    }

    setupTokenVisibilityToggles() {
        const pairs = [
            { inputId: 'github-token', toggleId: 'github-token-toggle' },
            { inputId: 'gitlab-token', toggleId: 'gitlab-token-toggle' },
        ];
        pairs.forEach(({ inputId, toggleId }) => {
            const input = document.getElementById(inputId);
            const toggle = document.getElementById(toggleId);
            if (!input || !toggle) return;
            toggle.addEventListener('click', () => {
                const isPassword = input.type === 'password';
                input.type = isPassword ? 'text' : 'password';
                toggle.classList.toggle('revealed', isPassword);
                toggle.title = isPassword ? 'Hide token' : 'Show token';
                toggle.setAttribute('aria-label', isPassword ? 'Hide token' : 'Show token');
            });
        });
    }

    clearStoredTokens() {
        localStorage.removeItem(TOKEN_STORAGE_KEYS.github);
        localStorage.removeItem(TOKEN_STORAGE_KEYS.gitlab);
        const storeCheckbox = document.getElementById('store-tokens');
        if (storeCheckbox) storeCheckbox.checked = false;
        localStorage.setItem(STORAGE_ENABLED_KEY, '0');
        const githubEl = document.getElementById('github-token');
        const gitlabEl = document.getElementById('gitlab-token');
        if (githubEl) githubEl.value = '';
        if (gitlabEl) gitlabEl.value = '';
    }

    async loadTokensFromStorage() {
        const githubEl = document.getElementById('github-token');
        const gitlabEl = document.getElementById('gitlab-token');
        const storeCheckbox = document.getElementById('store-tokens');

        const hasStoredTokens = localStorage.getItem(TOKEN_STORAGE_KEYS.github) || localStorage.getItem(TOKEN_STORAGE_KEYS.gitlab);
        const preference = localStorage.getItem(STORAGE_ENABLED_KEY);
        const enabled = preference === '1' || (preference === null && hasStoredTokens);
        if (storeCheckbox) storeCheckbox.checked = enabled;
        if (enabled && !preference) localStorage.setItem(STORAGE_ENABLED_KEY, '1');

        if (!enabled) return;

        for (const [name, storageKey] of Object.entries(TOKEN_STORAGE_KEYS)) {
            const el = name === 'github' ? githubEl : gitlabEl;
            const raw = localStorage.getItem(storageKey);
            if (!raw) continue;

            let plain;
            if (isEncryptedPayload(raw)) {
                plain = await decryptToken(raw);
            } else {
                plain = raw;
                try {
                    const encrypted = await encryptToken(plain);
                    localStorage.setItem(storageKey, encrypted);
                } catch (_) {
                    /* keep plain if encryption fails */
                }
            }
            if (plain) el.value = plain;
        }
    }

    /**
     * Persists current token input values to localStorage when storage is enabled.
     * Call this on form submit so tokens are saved even if the user never blurred the field.
     */
    async persistTokensFromForm() {
        const storeCheckbox = document.getElementById('store-tokens');
        if (!storeCheckbox?.checked) return;

        const githubEl = document.getElementById('github-token');
        const gitlabEl = document.getElementById('gitlab-token');
        if (!githubEl || !gitlabEl) return;

        for (const [name, storageKey] of Object.entries(TOKEN_STORAGE_KEYS)) {
            const el = name === 'github' ? githubEl : gitlabEl;
            const value = el.value?.trim();
            if (value) {
                try {
                    const encrypted = await encryptToken(value);
                    localStorage.setItem(storageKey, encrypted);
                } catch (_) {
                    localStorage.setItem(storageKey, value);
                }
            } else {
                localStorage.removeItem(storageKey);
            }
        }
    }

    setupTokenPersistence() {
        const storeCheckbox = document.getElementById('store-tokens');

        document.getElementById('github-token').addEventListener('blur', async (e) => {
            const value = e.target.value?.trim();
            if (!storeCheckbox?.checked) return;
            if (value) {
                try {
                    const encrypted = await encryptToken(value);
                    localStorage.setItem(TOKEN_STORAGE_KEYS.github, encrypted);
                } catch (_) {
                    localStorage.setItem(TOKEN_STORAGE_KEYS.github, value);
                }
            } else {
                localStorage.removeItem(TOKEN_STORAGE_KEYS.github);
            }
        });

        document.getElementById('gitlab-token').addEventListener('blur', async (e) => {
            const value = e.target.value?.trim();
            if (!storeCheckbox?.checked) return;
            if (value) {
                try {
                    const encrypted = await encryptToken(value);
                    localStorage.setItem(TOKEN_STORAGE_KEYS.gitlab, encrypted);
                } catch (_) {
                    localStorage.setItem(TOKEN_STORAGE_KEYS.gitlab, value);
                }
            } else {
                localStorage.removeItem(TOKEN_STORAGE_KEYS.gitlab);
            }
        });

        if (storeCheckbox) {
            storeCheckbox.addEventListener('change', () => {
                if (storeCheckbox.checked) {
                    localStorage.setItem(STORAGE_ENABLED_KEY, '1');
                } else {
                    localStorage.setItem(STORAGE_ENABLED_KEY, '0');
                    localStorage.removeItem(TOKEN_STORAGE_KEYS.github);
                    localStorage.removeItem(TOKEN_STORAGE_KEYS.gitlab);
                }
            });
        }
    }

    async handleSubmit(e) {
        e.preventDefault();

        const formData = new FormData(this.form);
        const url = formData.get('url');
        const github_token = formData.get('github_token');
        const gitlab_token = formData.get('gitlab_token');

        // Persist both tokens to localStorage on submit (in case user never blurred the field)
        await this.persistTokensFromForm();

        this.hideError();
        this.setLoading(true);

        try {
            const response = await fetch('/api/v1/analyze', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    url,
                    github_token,
                    gitlab_token
                }),
            });

            const data = await response.json();

            if (!response.ok || data.status === 'error') {
                throw new Error(data.message || 'Analysis failed');
            }

            this.currentGraphId = data.id;
            await this.loadAndDisplayGraph(data.id);
        } catch (error) {
            this.showError(error.message);
        } finally {
            this.setLoading(false);
        }
    }

    async loadAndDisplayGraph(graphId) {
        try {
            const response = await fetch(`/api/v1/graph/${graphId}`);
            if (!response.ok) {
                throw new Error('Failed to load graph');
            }

            const graphData = await response.json();
            this.currentGraphData = graphData;
            this.showGraph(graphData);
        } catch (error) {
            this.showError(error.message);
        }
    }

    showGraph(graphData) {
        this.formSection.classList.add('hidden');
        this.graphSection.classList.remove('hidden');

        try {
            const container = document.getElementById('cy');
            this.cy = renderGraph(container, graphData, (nodeData) => {
                this.showNodeDetails(nodeData);
            });
            setupBuildModal(this);
            this.updateLocalBranchUI(graphData);
            this.updateCABundleUI(graphData);
        } catch (error) {
            this.showGraphError(error.message);
        }
    }

    updateLocalBranchUI(graphData) {
        if (this.localBranchBadge && this.localBranchValue) {
            if (graphData.local_branch) {
                this.localBranchValue.textContent = graphData.local_branch;
                this.localBranchBadge.classList.remove('hidden');
            } else {
                this.localBranchBadge.classList.add('hidden');
            }
        }
    }

    updateCABundleUI(graphData) {
        const isLocal = !!graphData.local_branch;
        const hasValidBundle = !isLocal && graphData.ca_bundle && graphData.ca_bundle_valid !== false;

        this.downloadCABundleBtn.disabled = !hasValidBundle;
        this.downloadCABundleBtn.title = hasValidBundle
            ? 'Download CA bundle for Argo CD'
            : (isLocal ? 'CA bundle not used for local repositories' : 'CA bundle unavailable');
        this.downloadCABundleBtn.classList.toggle('hidden', isLocal);

        if (isLocal || !(graphData.ca_bundle_valid === false && graphData.ca_bundle_error)) {
            this.caBundleWarningDiv.classList.add('hidden');
        } else {
            this.caBundleWarningMessage.textContent = graphData.ca_bundle_error;
            this.caBundleWarningDiv.classList.remove('hidden');
        }
    }

    showGraphError(message) {
        this.graphErrorMessage.textContent = message;
        this.graphErrorDiv.classList.remove('hidden');
    }

    hideGraphError() {
        this.graphErrorDiv.classList.add('hidden');
    }

    async runBuild(nodeId, nodeLabel) {
        const buildLoading = document.getElementById('build-loading');
        const buildModal = document.getElementById('build-modal');
        const buildModalTitle = document.getElementById('build-modal-title');
        const buildModalContent = document.getElementById('build-modal-content');
        if (!nodeId || !this.currentGraphId || !buildLoading || !buildModal) return;
        buildLoading.classList.remove('hidden');
        const githubToken = document.getElementById('github-token')?.value?.trim() || '';
        const gitlabToken = document.getElementById('gitlab-token')?.value?.trim() || '';
        try {
            const res = await fetch(`/api/v1/node/${this.currentGraphId}/${encodeURIComponent(nodeId)}/build`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ github_token: githubToken, gitlab_token: gitlabToken }),
            });
            const data = await res.json();
            buildLoading.classList.add('hidden');
            if (!res.ok) {
                buildModalTitle.textContent = 'Build failed';
                buildModalContent.textContent = data.message || res.statusText || 'Unknown error';
                buildModal.classList.remove('hidden');
                return;
            }
            buildModalTitle.textContent = `Build: ${nodeLabel}`;
            buildModalContent.textContent = data.yaml || '';
            buildModal.classList.remove('hidden');
        } catch (err) {
            buildLoading.classList.add('hidden');
            buildModalTitle.textContent = 'Build failed';
            buildModalContent.textContent = err.message || 'Network error';
            buildModal.classList.remove('hidden');
        }
    }

    showForm() {
        this.graphSection.classList.add('hidden');
        this.formSection.classList.remove('hidden');
        this.hideSidebar();
        this.hideGraphError();
        this.caBundleWarningDiv?.classList.add('hidden');

        if (this.cy) {
            this.cy.destroy();
            this.cy = null;
        }
    }

    async showNodeDetails(nodeData) {
        this.sidebar.classList.remove('hidden');
        this.graphContainer.classList.add('sidebar-open');
        if (this.cy) this.cy.resize();

        // Afficher un état de chargement
        this.sidebarContent.innerHTML = `
        <h2>${nodeData.label || nodeData.id}</h2>
        <p>Loading details...</p>
    `;

        try {
            // Appel API pour obtenir les détails complets
            const encodedNodeId = encodeURIComponent(nodeData.id);
            const response = await fetch(`/api/v1/node/${this.currentGraphId}/${encodedNodeId}`);

            if (!response.ok) {
                throw new Error('Failed to load node details');
            }

            const nodeDetails = await response.json();

            // Build overlay button: only for directories (overlay/resource dirs), not single .yaml/.yml files or components
            const pathIsFile = (p) => p && (p.toLowerCase().endsWith('.yaml') || p.toLowerCase().endsWith('.yml'));
            const canBuild = nodeDetails.type !== 'component' && nodeDetails.type !== 'error' && !pathIsFile(nodeDetails.path);
            const buildButtonHtml = canBuild
                ? `<p class="node-info-actions"><button type="button" class="build-overlay-btn" data-node-id="${nodeDetails.id}" data-node-label="${nodeDetails.label || nodeDetails.id}">Build overlay</button></p>`
                : '';

            // Afficher les détails complets
            let html = `
            <h2>${nodeDetails.label || nodeDetails.id}</h2>
            <div class="node-info">
                <p><strong>ID:</strong> ${nodeDetails.id}</p>
                <p><strong>Type:</strong> <span class="badge badge-${nodeDetails.type}">${nodeDetails.type}</span></p>
                ${nodeDetails.path ? `<p><strong>Path:</strong> <code>${nodeDetails.path}</code></p>` : ''}
                ${buildButtonHtml}
            </div>
        `;

            // Relations - Parents
            if (nodeDetails.parents && nodeDetails.parents.length > 0) {
                html += '<div class="relations-section">';
                html += '<h3>⬆️ Parents</h3>';
                html += '<ul class="node-list">';
                nodeDetails.parents.forEach(parentId => {
                    html += `<li><a href="#" class="node-link" data-node-id="${parentId}">${this.getNodeLabel(parentId)}</a></li>`;
                });
                html += '</ul>';
                html += '</div>';
            }

            // Relations - Children
            if (nodeDetails.children && nodeDetails.children.length > 0) {
                html += '<div class="relations-section">';
                html += '<h3>⬇️ Children</h3>';
                html += '<ul class="node-list">';
                nodeDetails.children.forEach(childId => {
                    html += `<li><a href="#" class="node-link" data-node-id="${childId}">${this.getNodeLabel(childId)}</a></li>`;
                });
                html += '</ul>';
                html += '</div>';
            }

            // Content
            if (nodeDetails.content && Object.keys(nodeDetails.content).length > 0) {
                html += '<div class="content-section">';
                html += '<h3>📄 Content</h3>';
                html += `<pre><code>${JSON.stringify(nodeDetails.content, null, 2)}</code></pre>`;
                html += '</div>';
            }

            this.sidebarContent.innerHTML = html;

            // Build overlay button
            this.sidebarContent.querySelectorAll('.build-overlay-btn').forEach(btn => {
                btn.addEventListener('click', () => {
                    const nodeId = btn.dataset.nodeId;
                    const nodeLabel = btn.dataset.nodeLabel || 'Build';
                    if (nodeId) this.runBuild(nodeId, nodeLabel);
                });
            });

            // Ajouter les event listeners pour les liens de nodes
            this.attachNodeLinkListeners();

        } catch (error) {
            this.sidebarContent.innerHTML = `
            <h2>${nodeData.label || nodeData.id}</h2>
            <div class="error-message">
                <p>⚠️ Failed to load node details: ${error.message}</p>
            </div>
        `;
        }
    }

    // Nouvelle méthode pour obtenir le label d'un node à partir de son ID
    getNodeLabel(nodeId) {
        if (!this.currentGraphData) return nodeId;

        const element = this.currentGraphData.elements.find(
            el => el.group === 'nodes' && el.data.id === nodeId
        );

        return element ? (element.data.label || nodeId) : nodeId;
    }

    // Nouvelle méthode pour gérer les clics sur les liens de nodes
    attachNodeLinkListeners() {
        const nodeLinks = this.sidebarContent.querySelectorAll('.node-link');
        nodeLinks.forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const nodeId = e.target.dataset.nodeId;

                // Sélectionner et centrer le node dans le graphe
                if (this.cy) {
                    const node = this.cy.getElementById(nodeId);
                    if (node.length > 0) {
                        // Désélectionner tous les nodes
                        this.cy.elements().unselect();
                        // Sélectionner le node cible
                        node.select();
                        // Centrer la vue sur le node
                        this.cy.animate({
                            center: { eles: node },
                            zoom: 1.5
                        }, {
                            duration: 500
                        });
                        // Afficher les détails du node
                        this.showNodeDetails(node.data());
                    }
                }
            });
        });
    }


    hideSidebar() {
        this.sidebar.classList.add('hidden');
        this.graphContainer.classList.remove('sidebar-open');
        if (this.cy) {
            this.cy.elements().unselect();
            this.cy.resize();
        }
    }

    fitGraph() {
        if (this.cy) {
            this.cy.fit();
        }
    }

    centerGraph() {
        if (this.cy) {
            this.cy.center();
        }
    }

    exportPNG() {
        if (!this.cy) return;

        const blob = this.cy.png({ full: true, scale: 1, output: 'blob' });
        if (!blob) return;
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `kustomize-graph-${this.currentGraphId || 'export'}.png`;
        link.click();
        URL.revokeObjectURL(url);
    }

    exportSVG() {
        if (!this.cy) return;

        const svgContent = this.cy.svg({ scale: 1, full: true });
        const blob = new Blob([svgContent], { type: 'image/svg+xml' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `kustomize-graph-${this.currentGraphId || 'export'}.svg`;
        link.click();
        URL.revokeObjectURL(url);
    }

    async exportMermaid() {
        if (!this.currentGraphId) return;

        try {
            const res = await fetch(`/api/v1/graph/${this.currentGraphId}?format=mermaid`);
            if (!res.ok) throw new Error(res.statusText);
            const text = await res.text();
            const blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = `kustomize-graph-${this.currentGraphId}.mmd`;
            link.click();
            URL.revokeObjectURL(url);
        } catch (e) {
            this.showError('Failed to export Mermaid: ' + (e.message || String(e)));
        }
    }

    async downloadCABundle() {
        if (!this.currentGraphId) return;

        try {
            const res = await fetch(`/api/v1/graph/${this.currentGraphId}/ca-bundle`);
            if (res.status === 404) {
                this.showError('No CA bundle available for this graph');
                return;
            }
            if (!res.ok) throw new Error(res.statusText);
            const text = await res.text();
            const blob = new Blob([text], { type: 'application/x-pem-file' });
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = `ca-bundle-${this.currentGraphId}.pem`;
            link.click();
            URL.revokeObjectURL(url);
        } catch (e) {
            this.showError('Failed to download CA bundle: ' + (e.message || String(e)));
        }
    }

    setLoading(loading) {
        this.analyzeBtn.disabled = loading;
        this.analyzeBtn.textContent = loading ? 'Analyzing...' : 'Analyze';
    }

    showError(message) {
        this.errorMessage.textContent = message;
        this.errorDiv.classList.remove('hidden');
    }

    hideError() {
        this.errorDiv.classList.add('hidden');
    }
}

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    new App();
});
