/**
 * Go Notebook front-end glue.
 *
 * - Turns every textarea.code-editor into a CodeMirror editor (Go / markdown).
 * - Keeps the underlying textarea in sync so the server can read its value.
 * - Shift+Enter / Ctrl+Enter runs the cell.
 * - Light/dark theme toggle persisted in localStorage.
 * (The initial notebook load is triggered server side by the layout.)
 */
(function () {
    'use strict';

    var THEME_KEY = 'gonotebook-theme';
    var CM_THEMES = { light: 'idea', dark: 'material-darker' };

    // ------------------------------------------------------------------ theme
    function currentTheme() {
        var saved = localStorage.getItem(THEME_KEY);
        if (saved === 'light' || saved === 'dark') {
            return saved;
        }
        return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
            ? 'dark' : 'light';
    }

    function applyTheme(theme) {
        document.documentElement.setAttribute('data-theme', theme);
        document.querySelectorAll('.CodeMirror').forEach(function (el) {
            if (el.CodeMirror) {
                el.CodeMirror.setOption('theme', CM_THEMES[theme]);
            }
        });
    }

    window.nbToggleTheme = function () {
        var next = currentTheme() === 'dark' ? 'light' : 'dark';
        localStorage.setItem(THEME_KEY, next);
        applyTheme(next);
    };

    // ------------------------------------------------------- notebook manager
    window.nbCreate = function () {
        var n = prompt('New notebook name:');
        if (n) {
            window.send_event('link_reload', 'Create', n);
        }
    };

    window.nbRename = function (oldName) {
        var n = prompt('Rename notebook "' + oldName + '" to:', oldName);
        if (n && n !== oldName) {
            window.send_event('link_reload', 'Rename', JSON.stringify({ old: oldName, new: n }));
        }
    };

    window.nbDelete = function (name) {
        if (confirm('Delete notebook "' + name + '"? This cannot be undone.')) {
            window.send_event('link_reload', 'Delete', name);
        }
    };

    window.nbExport = function () {
        var chip = document.getElementById('notebook_name');
        var name = chip ? chip.textContent.trim() : '';
        if (name) {
            window.open('/export/' + encodeURIComponent(name), '_blank');
        }
    };

    window.nbToggleSidebar = function () {
        var sb = document.getElementById('sidebar');
        if (sb) {
            sb.classList.toggle('sidebar--hidden');
        }
    };

    window.nbToggleVars = function () {
        var p = document.getElementById('vars_panel');
        if (p) {
            p.classList.toggle('vars-panel--hidden');
        }
    };

    applyTheme(currentTheme());

    // ---------------------------------------------------------------- editors
    function runCell(textarea) {
        var cellId = textarea.getAttribute('data-cell');
        if (cellId && typeof window.send_event === 'function') {
            window.send_event(cellId, 'Click');
        }
    }

    function initEditor(textarea) {
        if (textarea.dataset.cmReady || typeof CodeMirror === 'undefined') {
            return;
        }
        textarea.dataset.cmReady = '1';

        var isGo = textarea.getAttribute('data-mode') === 'text/x-go';
        var cm = CodeMirror.fromTextArea(textarea, {
            mode: textarea.getAttribute('data-mode') || 'text/x-go',
            theme: CM_THEMES[currentTheme()],
            lineNumbers: isGo,
            indentUnit: isGo ? 4 : 2,
            tabSize: 4,
            indentWithTabs: isGo,
            matchBrackets: true,
            autoCloseBrackets: true,
            viewportMargin: Infinity,
            extraKeys: {
                'Shift-Enter': function (instance) {
                    instance.save();
                    runCell(textarea);
                },
                'Ctrl-Enter': function (instance) {
                    instance.save();
                    runCell(textarea);
                },
                'Cmd-Enter': function (instance) {
                    instance.save();
                    runCell(textarea);
                }
            }
        });

        // Keep the hidden textarea in sync: the server reads its .value
        // through the LiveView "get" protocol before evaluating the cell.
        cm.on('change', function () {
            cm.save();
        });
    }

    function initEditors(root) {
        (root || document).querySelectorAll('textarea.code-editor').forEach(initEditor);
    }

    // Cells are mounted and re-rendered over WebSocket: watch the DOM and
    // upgrade any textarea that appears.
    var sidebarBooted = false;
    var nbRestored = false;
    var observer = new MutationObserver(function () {
        initEditors(document);
        // On small screens the sidebar starts collapsed (it arrives with the
        // layout over the WebSocket, after this script boots).
        if (!sidebarBooted) {
            var sb = document.getElementById('sidebar');
            if (sb) {
                sidebarBooted = true;
                if (window.innerWidth <= 900) {
                    sb.classList.add('sidebar--hidden');
                }
            }
        }
        // Reopen the notebook this browser was using. The server loads the
        // default notebook first; once the name chip is filled the drivers
        // are live, so this cannot race the connection handshake.
        if (!nbRestored) {
            var chip = document.getElementById('notebook_name');
            var current = chip ? chip.textContent.trim() : '';
            if (current) {
                nbRestored = true;
                var saved = localStorage.getItem('gonotebook-nb');
                if (saved && saved !== current) {
                    window.send_event('link_reload', 'Open', saved);
                }
            }
        }
    });

    // Click on an output label collapses/expands that output (Jupyter style).
    document.addEventListener('click', function (e) {
        var label = e.target.closest ? e.target.closest('.output-label') : null;
        if (label) {
            var out = label.closest('.cell-output');
            if (out) {
                out.classList.toggle('output--collapsed');
            }
        }
    });

    function boot() {
        var content = document.getElementById('content');
        if (content) {
            observer.observe(content, { childList: true, subtree: true });
        }
        initEditors(document);
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', boot);
    } else {
        boot();
    }
})();
