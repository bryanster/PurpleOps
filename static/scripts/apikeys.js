var pendingDeleteID = null;

function newKeyModal() {
    document.getElementById('apiKeyForm').action = '/api-keys';
    $('#apiKeyDetailModal').modal('show');
}

function copyKey() {
    var el = document.getElementById('revealedKey');
    el.select();
    document.execCommand('copy');
}

function timeFormatter(value) {
    if (!value || value === '-|') return '-';
    return value.split('|')[0] || '-';
}

function keyActionFormatter(value) {
    return '<button class="btn btn-danger btn-sm" onclick="confirmDeleteKey(this)" data-name="' + value + '"><i class="bi-trash"></i></button>';
}

function confirmDeleteKey(btn) {
    var row = $(btn).closest('tr');
    var rowData = $('#keyTable').bootstrapTable('getData')[row.data('index')];
    pendingDeleteID = rowData.id;
    const warningEl = document.getElementById('deleteKeyWarning');
    warningEl.textContent = '';
    warningEl.append('Revoke API key ');
    const strong = document.createElement('strong');
    strong.textContent = rowData.name;
    warningEl.append(strong);
    warningEl.append('? This cannot be undone.');
    document.getElementById('deleteKeyButton').onclick = function() { doDeleteKey(rowData.id); };
    $('#deleteKeyModal').modal('show');
}

function doDeleteKey(id) {
    apiFetch('/api-keys/' + id, 'DELETE')
        .then(function(r) {
            if (r.ok) {
                $('#deleteKeyModal').modal('hide');
                location.reload();
            } else {
                r.text().then(function(t) { alert('Failed to revoke key: ' + t); });
            }
        });
}

$(function() {
    $('#keyTable').show();

    document.getElementById('apiKeyDetailButton').addEventListener('click', function(e) {
        e.preventDefault();
        var form = document.getElementById('apiKeyForm');
        apiFetch('/api-keys', 'POST', new URLSearchParams(new FormData(form)))
            .then(function(resp) {
                if (!resp.ok) return resp.text().then(function(t) { throw new Error(t); });
                return resp.json();
            })
            .then(function(data) {
                $('#apiKeyDetailModal').modal('hide');
                document.getElementById('revealedKey').value = data.key;
                $('#keyRevealModal').one('hidden.bs.modal', function() { location.reload(); });
                $('#keyRevealModal').modal('show');
            })
            .catch(function(err) { alert('Error: ' + err.message); });
    });
});
