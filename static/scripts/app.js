// Shared utilities — loaded on every page via master.html.

// TODO: wire up real CSRF protection — inject a token into the template context,
// emit it as <meta name="csrf-token" content="..."> in master.html, and verify
// it server-side. Until then csrfToken() returns '' and the X-CSRF-Token header
// is sent but meaningless.
function csrfToken() {
    var el = document.querySelector('meta[name="csrf-token"]');
    return el ? el.content : '';
}

// apiFetch wraps fetch() with consistent CSRF header and JSON body serialisation.
// body can be FormData, URLSearchParams (sent as-is) or a plain object (sent as JSON).
function apiFetch(url, method, body) {
    var opts = {
        method: method || 'GET',
        headers: { 'X-CSRF-Token': csrfToken() }
    };
    if (body != null) {
        if (body instanceof FormData || body instanceof URLSearchParams) {
            opts.body = body;
        } else {
            opts.headers['Content-Type'] = 'application/json';
            opts.body = JSON.stringify(body);
        }
    }
    return fetch(url, opts);
}

// utcToLocal converts a UTC timestamp string to the browser's local time,
// formatted as "YYYY-MM-DDTHH:MM".
function utcToLocal(utc) {
    if (!utc) return '';
    var d = new Date(utc.replace(' ', 'T') + 'Z');
    return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
}

// bgFormatter returns a bootstrap-table cell-style object based on a score or
// state string. Used by any table column that needs colour-coded values.
function bgFormatter(value) {
    var bg = '', text = '';
    if (['Missed', '1.0', '1.5'].includes(value)) {
        bg = 'danger';
    } else if (['Running', 'Logged', '2.0', '2.5'].includes(value)) {
        bg = 'warning';
    } else if (['Alerted', '3.0', '3.5'].includes(value)) {
        bg = 'success';
    } else if (['Prevented', '4.0', '4.5', '5.0'].includes(value)) {
        bg = 'info'; text = 'light';
    } else if (value === 'Complete') {
        bg = 'primary'; text = 'light';
    } else if (value === 'Pending') {
        bg = 'light';
    } else if (['False', false, '0.0', '0.5'].includes(value)) {
        bg = 'dark'; text = 'light';
    }
    var css = { background: 'var(--bs-' + bg + ')' };
    if (text) css.color = 'var(--bs-' + text + ')';
    return { css: css };
}
