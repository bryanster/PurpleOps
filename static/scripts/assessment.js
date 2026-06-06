function escAttr(str) {
    return String(str).replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/'/g,'&#39;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

// Onload
$(function () {
    // The cookie extension sometimes force-shows the ID column, so hide it.
    $('#assessmentTable').bootstrapTable('hideColumn', 'id');
    $('#assessmentTable').show();
});

var row = null;
var rowData = null;

// Pop modal when adding a new raw testcase and clear old data.
$('#newTestcase').click(function() {
    $("#newTestcaseForm").trigger('reset');
    $('#newTestcaseModal').modal('show');
});

// Auto-populate tactic when MitreID changes in the new testcase modal.
$('#newTestcaseForm #mitreid').on('changed.bs.select change', function() {
    var selectedMitre = $(this).val();
    if (selectedMitre && typeof mitreTactics !== 'undefined' && mitreTactics[selectedMitre]) {
        $('#newTestcaseForm #tactic').val(mitreTactics[selectedMitre]);
    }
});

// Build a bootstrap-table row object from a server response.
function formatRow(r) {
    return {
        add:          r.add,
        id:           r.id,
        mitreid:      r.mitreid,
        name:         r.name,
        tactic:       r.tactic,
        state:        r.state,
        visible:      r.visible,
        tags:         r.tags.join(","),
        uuid:         r.uuid,
        start:        r.starttime && r.starttime !== "None" ? r.starttime : "",
        modified:     r.modifytime,
        preventscore: r.preventedrating != null ? r.preventedrating : "",
        detectscore:  r.detectionrating != null ? r.detectionrating : "",
        outcome:      r.outcome,
        actions:      "",
    };
}

function tableUpdate(body) {
    $('#assessmentTable').bootstrapTable('updateByUniqueId', { id: body.id, row: formatRow(body), replace: true });
}

// New raw testcase.
$("#newTestcaseForm").submit(function(e) {
    e.preventDefault();
    apiFetch(e.target.action, 'POST', new URLSearchParams(new FormData(e.target)))
        .then(function(r) { return r.json(); })
        .then(function(body) {
            $('#assessmentTable').bootstrapTable('append', [formatRow(body)]);
            $('#newTestcaseModal').modal('hide');
        });
});

// Import testcases from template library.
$('#testcaseTemplatesButton').click(function() {
    var ids = $('#testcaseTemplateTable').bootstrapTable('getSelections').map(function(r) { return r.id; });
    var assessmentId = window.location.pathname.split("/").filter(Boolean)[1];
    apiFetch('/assessment/' + assessmentId + '/import/template', 'POST', { ids: ids })
        .then(function(r) { return r.json(); })
        .then(function(result) {
            result.forEach(function(r) { $('#assessmentTable').bootstrapTable('append', [formatRow(r)]); });
            $('#testcaseTemplatesModal').modal('hide');
        });
});

// Import from Navigator JSON file.
$("#navigatorTemplateForm").submit(function(e) {
    e.preventDefault();
    apiFetch(e.target.action, 'POST', new FormData(e.target))
        .then(function(r) { return r.json(); })
        .then(function(body) {
            body.forEach(function(r) { $('#assessmentTable').bootstrapTable('append', [formatRow(r)]); });
            $('#testcaseNavigatorModal').modal('hide');
        });
});

// Import from campaign template file.
$("#campaignTemplateForm").submit(function(e) {
    e.preventDefault();
    apiFetch(e.target.action, 'POST', new FormData(e.target))
        .then(function(r) { return r.json(); })
        .then(function(body) {
            body.forEach(function(r) { $('#assessmentTable').bootstrapTable('append', [formatRow(r)]); });
            $('#testcaseCampaignModal').modal('hide');
        });
});

// Row action helpers — all read row data at call time to avoid stale globals.

function visibleTest(event) {
    event.stopPropagation();
    var rowEl = $(event.target).closest("tr");
    var data = $('#assessmentTable').bootstrapTable('getData')[rowEl.data("index")];
    apiFetch('/testcase/' + data.id + '/toggle-visibility')
        .then(function(r) { return r.json(); })
        .then(tableUpdate);
}

function cloneTest(event) {
    event.stopPropagation();
    var rowEl = $(event.target).closest("tr");
    var data = $('#assessmentTable').bootstrapTable('getData')[rowEl.data("index")];
    apiFetch('/testcase/' + data.id + '/clone')
        .then(function(r) { return r.json(); })
        .then(function(body) {
            $('#assessmentTable').bootstrapTable('insertRow', { index: rowEl.data("index") + 1, row: formatRow(body) });
        });
}

function toggleTimer(event) {
    event.stopPropagation();
    var rowEl = $(event.target).closest("tr");
    var data = $('#assessmentTable').bootstrapTable('getData')[rowEl.data("index")];
    apiFetch('/testcase/' + data.id + '/toggle-timer')
        .then(function(r) { return r.json(); })
        .then(tableUpdate);
}

function deleteTest(event) {
    event.stopPropagation();
    var rowEl = $(event.target).closest("tr");
    var data = $('#assessmentTable').bootstrapTable('getData')[rowEl.data("index")];
    apiFetch('/testcase/' + data.id + '/delete')
        .then(function(r) {
            if (r.ok) $('#assessmentTable').bootstrapTable('removeByUniqueId', data.id);
        });
}

// Table formatters.

function nameFormatter(name, row) {
    return '<a href="/testcase/' + row.id + '">' + name + '</a>';
}

function visibleFormatter(value) {
    return (value === "True" || value === true) ? "✅" : "❌";
}

function tagFormatter(tags) {
    if (!tags) return '';
    return tags.split(",").map(function(tag) {
        var parts = tag.split("|");
        return '<span class="badge rounded-pill" style="background:' + parts[1] + ';cursor:pointer">' + parts[0] + '</span>';
    }).join("&nbsp;");
}

function actionFormatter(value, row) {
    var timerBtn = '';
    if (row.state === 'Pending') {
        timerBtn = '<button type="button" class="btn btn-success py-0" onclick="toggleTimer(event)" title="Start Timer"><i class="bi-play-fill">&zwnj;</i></button>';
    } else if (row.state === 'Running') {
        timerBtn = '<button type="button" class="btn btn-danger py-0" onclick="toggleTimer(event)" title="Stop Timer"><i class="bi-stop-fill">&zwnj;</i></button>';
    } else if (row.state === 'Complete') {
        timerBtn = '<button type="button" class="btn btn-warning py-0" onclick="toggleTimer(event)" title="Restart Timer"><i class="bi-arrow-counterclockwise">&zwnj;</i></button>';
    }
    return '<div class="btn-group btn-group-sm" role="group">' +
        timerBtn +
        '<button type="button" class="btn btn-info py-0" onclick="visibleTest(event)" title="Toggle Visibility"><i class="bi-eye">&zwnj;</i></button>' +
        '<button type="button" class="btn btn-warning py-0" onclick="cloneTest(event)" title="Clone"><i class="bi-files">&zwnj;</i></button>' +
        '<button type="button" class="btn btn-danger py-0" onclick="deleteTest(event)" title="Delete"><i class="bi-trash-fill">&zwnj;</i></button>' +
        '</div>';
}

function timeFormatter(utc) {
    return utc ? utcToLocal(utc) : '';
}

// Multi-field management (datasources, rules, detection/prevention sources).

$('.multiNew').click(function(event) {
    var type = event.target.id.replace("NewButton", "");
    $('#' + type + 'Table').bootstrapTable("append", [{ id: 'tmp-' + Date.now(), name: "", description: "", delete: "" }]);
});

$('.assessmentMultiButton').click(function(event) {
    var type = event.target.id.replace("manage", "").replace("Button", "").toLowerCase();
    var assessmentId = window.location.pathname.split("/").filter(Boolean)[1];
    apiFetch('/assessment/' + assessmentId + '/multi/' + type, 'POST', { data: $('#' + type + 'Table').bootstrapTable("getData") })
        .then(function(r) { return r.json(); })
        .then(function(result) {
            result.forEach(function(r) { r.delete = ""; });
            $('#' + type + 'Table').bootstrapTable("load", result);
            $(event.target).closest(".modal").modal("hide");
        });
});

// Update table cell when name/description inputs change.
$('.multiTable').on('change', '.multi', function(event) {
    $(event.delegateTarget).bootstrapTable("updateCellByUniqueId", {
        id: $(event.target).closest("tr").data("uniqueid"),
        field: event.target.name,
        value: event.target.value
    });
});

function multiDeleteRow(event) {
    var tableId = $(event.target).closest("table")[0].id;
    $('#' + tableId).bootstrapTable("removeByUniqueId", $(event.target).closest("tr").data("uniqueid"));
}

function multiNameFormatter(val) {
    return '<input type="text" name="name" value="' + escAttr(val) + '" class="multi" placeholder="Name..."/>';
}
function multiDescriptionFormatter(val) {
    return '<input type="text" name="description" value="' + escAttr(val) + '" class="multi" placeholder="Description..."/>';
}
function multiDeleteFormatter() {
    return '<button type="button" class="btn btn-danger py-0" onclick="multiDeleteRow(event)" title="Delete"><i class="bi-trash-fill">&zwnj;</i></button>';
}

// Show count of selected testcases.
$('#assessmentTable').on('check.bs.table uncheck.bs.table check-all.bs.table uncheck-all.bs.table', function() {
    var count = $("#assessmentTable").bootstrapTable('getSelections').length;
    if (count > 0) {
        $("#selected-count").text("(" + count + " selected)").show();
    } else {
        $("#selected-count").hide();
    }
});
