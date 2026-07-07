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
    var CM_THEMES = { light: 'default', dark: 'material-darker' };

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
    var observer = new MutationObserver(function () {
        initEditors(document);
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
