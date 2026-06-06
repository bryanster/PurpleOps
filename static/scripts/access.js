// Delay table showing until page is loaded to prevent jumping.
$(function () {
    $('#userTable').show();
});

var row = null;
var rowData = null;

function newUserModal() {
    $("#userDetailForm").trigger('reset');
    $('#userDetailForm').attr('action', '/manage/access/user');
    $('#userDetailLabel').text("New User");
    $('#userDetailButton').text("Create");
    $('#password').attr("type", "text");
    $('#userDetailForm #roles').selectpicker('val', "");
    $('#userDetailForm #assessments').selectpicker('val', "");
    $('#userDetailModal').modal('show');
}

function editUserModal(el) {
    row = $(el).closest("tr");
    rowData = $('#userTable').bootstrapTable('getData')[row.data("index")];
    $("#userDetailForm").trigger('reset');
    $('#userDetailForm').attr('action', '/manage/access/user/' + rowData.id);
    $('#userDetailLabel').text("Edit User");
    $('#userDetailButton').text("Update");
    // Show placeholder password so it's clear editing details won't wipe the real one.
    $('#password').attr("type", "password").val(" ".repeat(128));
    $('#userDetailForm #username').val(rowData.username);
    $('#userDetailForm #email').val(rowData.email);
    $('#userDetailForm #roles').selectpicker('val', rowData.roles.split(", "));
    $('#userDetailForm #assessments').selectpicker('val', rowData.assessments.split(", "));
    $('#userDetailModal').modal('show');
}

function deleteUserModal(el) {
    row = $(el).closest("tr");
    rowData = $('#userTable').bootstrapTable('getData')[row.data("index")];
    $('#deleteUserForm').attr('action', '/manage/access/user/' + rowData.id);
    $('#deleteUserWarning').text('Really Delete ' + rowData.username + '?');
    $('#deleteUserModal').modal('show');
}

// New / edit user form — shared handler.
$("#userDetailForm").submit(function(e) {
    e.preventDefault();
    apiFetch(e.target.action, 'POST', new URLSearchParams(new FormData(e.target)))
        .then(function(r) { return r.json(); })
        .then(function(body) {
            var assessmentsCell;
            if (body.roles.includes("Admin")) {
                assessmentsCell = "*";
            } else if (body.assessments.length) {
                assessmentsCell = body.assessments.join(", ");
            } else {
                assessmentsCell = "-";
            }
            var newRow = {
                id:          body.id,
                username:    body.username,
                email:       body.email,
                roles:       body.roles.length ? body.roles.join(", ") : "-",
                assessments: assessmentsCell,
                actions:     body.username,
            };
            if ($('#userTable').bootstrapTable('getRowByUniqueId', body.id)) {
                $('#userTable').bootstrapTable('updateRow', { index: row.data("index"), row: newRow });
            } else {
                $('#userTable').bootstrapTable('append', [newRow]);
            }
            $('#userDetailModal').modal('hide');
        });
});

// Delete user.
$('#deleteUserButton').click(function() {
    apiFetch('/manage/access/user/' + rowData.id, 'DELETE')
        .then(function(r) {
            if (r.ok) {
                $('#userTable').bootstrapTable('removeByUniqueId', rowData.id);
                $('#deleteUserModal').modal('hide');
            }
        });
});

function actionFormatter(username) {
    var deleteBtn = username !== 'admin'
        ? '<button type="button" class="btn btn-danger py-0" title="Delete" onclick="deleteUserModal(this)"><i class="bi-trash">&zwnj;</i></button>'
        : '';
    return '<div class="btn-group btn-group-sm" role="group">' +
        '<button type="button" class="btn btn-primary py-0" title="Edit" onclick="editUserModal(this)"><i class="bi-pencil-fill">&zwnj;</i></button>' +
        deleteBtn +
        '</div>';
}

// Login time column — pipe-delimited "utc|ip" string from the server.
function timeFormatter(utcip) {
    var parts = utcip.split("|");
    var utc = parts[0], ip = parts[1];
    if (!utc || utc === "-") return "-";
    return utcToLocal(utc) + (ip ? ' (' + ip + ')' : '');
}
