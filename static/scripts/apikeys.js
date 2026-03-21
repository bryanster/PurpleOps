function csrfToken() {
	var el = document.querySelector('meta[name="csrf-token"]');
	return el ? el.content : '';
}

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
  var parts = value.split('|');
  return parts[0] || '-';
}

function keyActionFormatter(value) {
  return '<button class="btn btn-danger btn-sm" onclick="confirmDeleteKey(this)" data-name="' + value + '"><i class="bi-trash"></i></button>';
}

function confirmDeleteKey(btn) {
  var row = $(btn).closest('tr');
  var rowData = $('#keyTable').bootstrapTable('getData')[row.data('index')];
  pendingDeleteID = rowData.id;
  document.getElementById('deleteKeyWarning').innerHTML = 'Revoke API key <strong>' + rowData.name + '</strong>? This cannot be undone.';
  document.getElementById('deleteKeyButton').onclick = function() { doDeleteKey(rowData.id); };
  $('#deleteKeyModal').modal('show');
}

function doDeleteKey(id) {
  $.ajax({
    url: '/api-keys/' + id,
    type: 'DELETE',
    headers: { 'X-CSRF-Token': csrfToken() },
    success: function() {
      $('#deleteKeyModal').modal('hide');
      location.reload();
    },
    error: function(xhr) {
      alert('Failed to revoke key: ' + xhr.responseText);
    }
  });
}

$(function() {
  $('#keyTable').show();

  document.getElementById('apiKeyDetailButton').addEventListener('click', function(e) {
    e.preventDefault();
    var form = document.getElementById('apiKeyForm');
    var data = new FormData(form);
    var params = new URLSearchParams();
    for (var pair of data.entries()) {
      params.append(pair[0], pair[1]);
    }
    fetch('/api-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded', 'X-CSRF-Token': csrfToken() },
      body: params.toString()
    }).then(function(resp) {
      if (!resp.ok) return resp.text().then(function(t) { throw new Error(t); });
      return resp.json();
    }).then(function(data) {
      $('#apiKeyDetailModal').modal('hide');
      document.getElementById('revealedKey').value = data.key;
      $('#keyRevealModal').one('hidden.bs.modal', function() { location.reload(); });
      $('#keyRevealModal').modal('show');
    }).catch(function(err) {
      alert('Error: ' + err.message);
    });
  });

});
