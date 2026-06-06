// Delay table showing until page is loaded to prevent jumping.
$(function () {
    $('#assessmentsTable').show();
});

var row = null;
var rowData = null;

$('#newAssessment').click(function() {
    $("#newAssessmentForm").trigger('reset');
    $('#newAssessmentForm').attr('action', '/assessment');
    $('#newAssessmentLabel').text("New Assessment");
    $('#newAssessmentButton').text("Create");
    $('#newAssessmentModal').modal('show');
});

function editAssessmentModal(el) {
    row = $(el).closest("tr");
    rowData = $('#assessmentsTable').bootstrapTable('getData')[row.data("index")];
    $("#newAssessmentForm").trigger('reset');
    $('#newAssessmentForm').attr('action', '/assessment/' + rowData.id);
    $('#newAssessmentLabel').text("Edit Assessment");
    $('#newAssessmentButton').text("Update");
    $('#newAssessmentForm #name').val(rowData.name);
    $('#newAssessmentForm #description').val(rowData.description);
    $('#newAssessmentModal').modal('show');
}

function deleteAssessmentModal(el) {
    row = $(el).closest("tr");
    rowData = $('#assessmentsTable').bootstrapTable('getData')[row.data("index")];
    $('#deleteAssessmentForm').attr('action', '/assessment/' + rowData.id);
    $('#deleteAssessmentWarning').text('Really Delete ' + rowData.name + '?');
    $('#deleteAssessmentModal').modal('show');
}

function assessmentRow(body) {
    return { id: body.id, name: body.name, description: body.description, progress: body.progress, actions: "" };
}

// New / edit assessment form — shared handler.
$("#newAssessmentForm").submit(function(e) {
    e.preventDefault();
    apiFetch(e.target.action, 'POST', new URLSearchParams(new FormData(e.target)))
        .then(function(r) { return r.json(); })
        .then(function(body) {
            var r = assessmentRow(body);
            if ($('#assessmentsTable').bootstrapTable('getRowByUniqueId', body.id)) {
                $('#assessmentsTable').bootstrapTable('updateRow', { index: row.data("index"), row: r, replace: true });
            } else {
                $('#assessmentsTable').bootstrapTable('append', [r]);
            }
            $('#newAssessmentModal').modal('hide');
            $('#newAssessmentForm').trigger('reset');
        });
});

// Import entire assessment.
$("#importAssessmentForm").submit(function(e) {
    e.preventDefault();
    apiFetch(e.target.action, 'POST', new FormData(e.target))
        .then(function(r) { return r.json(); })
        .then(function(body) {
            $('#assessmentsTable').bootstrapTable('append', [assessmentRow(body)]);
            $('#importAssessmentModal').modal('hide');
            $('#importAssessmentForm').trigger('reset');
        });
});

// Delete assessment.
$('#deleteAssessmentButton').click(function() {
    apiFetch('/assessment/' + rowData.id, 'DELETE')
        .then(function(r) {
            if (r.ok) {
                $('#assessmentsTable').bootstrapTable('removeByUniqueId', rowData.id);
                $('#deleteAssessmentModal').modal('hide');
            }
        });
});

function nameFormatter(name, row) {
    return '<a href="/assessment/' + row.id + '">' + name + '</a>';
}

function progressFormatter(progress) {
    var parts = progress.split('|');
    return '<div class="progress">' +
        '<div class="progress-bar bg-danger" role="progressbar" style="width:' + parts[0] + '%"></div>' +
        '<div class="progress-bar bg-warning" role="progressbar" style="width:' + parts[1] + '%"></div>' +
        '<div class="progress-bar bg-success" role="progressbar" style="width:' + parts[2] + '%"></div>' +
        '<div class="progress-bar bg-info"    role="progressbar" style="width:' + parts[3] + '%"></div>' +
        '</div>';
}

function actionFormatter() {
    return '<div class="btn-group btn-group-sm" role="group">' +
        '<button type="button" class="btn btn-primary py-0" title="Edit" onclick="editAssessmentModal(this)"><i class="bi-pencil">&zwnj;</i></button>' +
        '<button type="button" class="btn btn-danger py-0" title="Delete" onclick="deleteAssessmentModal(this)"><i class="bi-trash-fill">&zwnj;</i></button>' +
        '</div>';
}
