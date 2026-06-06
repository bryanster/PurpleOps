// When a source/target etc. "new" button is clicked, add an empty row.
$('.multiNew').click(function(event) {
    var type = event.target.id.replace("NewButton", "");
    var newRow = { id: 'tmp-' + Date.now(), name: "", delete: "" };
    if (type === "tags") { newRow.colour = ""; } else { newRow.description = ""; }
    $('#' + type + 'Table').bootstrapTable("append", [newRow]);
});

// Save a source/target etc. table via AJAX and refresh selects.
$('.multiButton').click(function(event) {
    // Derive the resource type from the button ID, e.g. "multiSourcesButton" → "sources".
    var type = event.target.id.replace("multi", "").replace("Button", "").toLowerCase() + "s";
    var assessmentUrl = $("#assessment-crumb-button").attr("href");
    apiFetch(assessmentUrl + '/multi/' + type, 'POST', { data: $('#' + type + 'Table').bootstrapTable("getData") })
        .then(function(r) { return r.json(); })
        .then(function(result) {
            result.forEach(function(r) { r.delete = ""; });
            $('#' + type + 'Table').bootstrapTable("load", result);
            $(event.target).closest(".modal").modal("hide");

            // Repopulate the corresponding selectpicker with updated options.
            var selectedIDs = $('#' + type).val();
            $(".dynopt-" + type).remove();
            result.forEach(function(i) {
                var selected = (selectedIDs || []).includes(i.id) ? "selected" : "";
                var pill = type === "tags"
                    ? 'data-content="<span class=\'badge rounded-pill\' style=\'background:' + i.colour + '\'>' + i.name + '</span>"'
                    : "";
                $('#' + type).append('<option ' + selected + ' class="dynopt-' + type + '" value="' + i.id + '" ' + pill + '>' + i.name + '</option>');
            });
            $('#' + type).selectpicker('refresh');
        });
});

// If "Manage" is chosen from a multi-select, open the manage modal instead.
$('.selectpicker').change(function(event) {
    var type = event.target.id;
    if ($('#' + type).val().includes("Manage")) {
        $(event.target).selectpicker('val', $('#' + type).val().filter(function(v) { return v !== "Manage"; }));
        $(event.target).selectpicker('toggle');
        $('#multi' + type[0].toUpperCase() + type.slice(1, -1) + 'Modal').modal('show');
    }
});

// When a source/target etc. name/description changes, sync the value into the table.
$('.multiTable').on('change', '.multi', function(event) {
    $(event.delegateTarget).bootstrapTable("updateCellByUniqueId", {
        id: $(event.target).closest("tr").data("uniqueid"),
        field: event.target.name,
        value: event.target.value
    });
});

function deleteMultiRow(event) {
    var tableId = $(event.target).closest("table")[0].id;
    $('#' + tableId).bootstrapTable("removeByUniqueId", $(event.target).closest("tr").data("uniqueid"));
}

// Multi-table cell formatters.
function nameFormatter(val) {
    return '<input type="text" name="name" value="' + val + '" class="multi" placeholder="Name..."/>';
}
function descriptionFormatter(val) {
    return '<input type="text" name="description" value="' + val + '" class="multi" placeholder="Description..."/>';
}
function colourFormatter(val) {
    return '<input type="color" name="colour" value="' + val + '" class="multi"/>';
}
function deleteFormatter() {
    return '<button type="button" class="btn btn-danger py-0" onclick="deleteMultiRow(event)" title="Delete"><i class="bi-trash-fill">&zwnj;</i></button>';
}

// Auto-populate tactic when MitreID changes.
$('#mitreid').on('changed.bs.select change', function() {
    var selectedMitre = $(this).val();
    if (selectedMitre && typeof mitreTactics !== 'undefined' && mitreTactics[selectedMitre]) {
        $('#tactic').val(mitreTactics[selectedMitre]);
    }
});

// Auto-resize textareas.
$('#objective, #actions, #rednotes, #bluenotes').on('input', function(event) {
    event.target.style.height = 0;
    event.target.style.height = (event.target.scrollHeight + 5) + 'px';
}).trigger('input');

// "Same as detection source" — sync prevention source to match detection source by name.
function syncPreventionToDetection() {
    var detectionName = $('#detectionsource option:selected').text().trim();
    var match = $('#preventionsource option').filter(function() {
        return $(this).text().trim() === detectionName;
    });
    $('#preventionsource').val(match.length ? match.val() : '');
}

$('#samesource').on('change', function() {
    if ($(this).is(':checked')) {
        syncPreventionToDetection();
        $('#preventionsource').addClass('text-muted').css('pointer-events', 'none');
    } else {
        $('#preventionsource').removeClass('text-muted').css('pointer-events', '');
    }
});

$('#detectionsource').on('change', function() {
    if ($('#samesource').is(':checked')) syncPreventionToDetection();
});

$('#preventionsource').on('change', function() {
    if ($('#samesource').is(':checked')) {
        var detectionName = $('#detectionsource option:selected').text().trim();
        if ($(this).find('option:selected').text().trim() !== detectionName) {
            $('#samesource').prop('checked', false);
            $('#preventionsource').removeClass('text-muted').css('pointer-events', '');
        }
    }
});

// Show/hide prevention fields based on "prevented" radio.
$('input[name="prevented"]').on('change', function() {
    var current = $('input[name="prevented"]:checked').val();
    if (["No", "N/A", ""].includes(current)) {
        $("#preventedrating").val(current.replace("No", "0.0"));
        $("#preventedrating-container, #preventionsource-container").hide();
        $("#preventionsource").val("");
        $("#samesource").prop("checked", false).trigger("change");
    } else {
        if (["0.0", "N/A"].includes($("#preventedrating").val())) $("#preventedrating").val("");
        $("#preventedrating-container, #preventionsource-container").show();
    }
}).trigger('change');

// Show/hide priority urgency field.
$('input[name="priority"]').on('change', function() {
    var current = $('input[name="priority"]:checked').val();
    if (current === "N/A") {
        $("#priorityurgency").val("N/A");
        $("#urgency-container").hide();
    } else {
        if ($("#priorityurgency").val() === "N/A") $("#priorityurgency").val("");
        $("#urgency-container").show();
    }
}).trigger('change');

// Show/hide detection/alert/logged fields based on "alerted" radio.
$('input[name="alerted"]').on('change', function() {
    var current = $('input[name="alerted"]:checked').val();
    if (current === "Yes") {
        $("#alert-container, #detection-container, #detectionsource-container").show();
        $("#logged-container").hide();
        $('input[name="logged"]').prop('checked', false);
        $('#log-yes').prop("checked", true);
        if ($('#detectionrating').val() === "0.0") $('#detectionrating').val("");
    } else if (current === "No") {
        $("#alert-container").hide();
        $("#alertseverity").val("");
        $("#logged-container").show();
        $("#detection-container").hide();
    } else {
        $("#alert-container, #logged-container, #detection-container, #detectionsource-container").hide();
        $("#detectionsource").val("");
    }
}).trigger('change');

// Show/hide detection fields based on "logged" radio.
$('input[name="logged"]').on('change', function() {
    var current = $('input[name="logged"]:checked').val();
    if (current === "Yes") {
        $('#detection-container, #detectionsource-container').show();
    } else if (current === "No") {
        $('#detectionrating').val("0.0");
        $('#detection-container').hide();
        if ($('input[name="alerted"]:checked').val() !== "Yes") {
            $('#detectionsource-container').hide();
            $('#detectionsource').val("");
        }
    }
}).trigger('change');

// Save testcase via AJAX and show toast on success.
$("#ttpform").submit(function(e) {
    e.preventDefault();
    var form = e.target;
    apiFetch(form.action, 'POST', new FormData(form))
        .then(function(response) {
            if (response.ok) {
                displayNewEvidence(new FormData(form));
                new bootstrap.Toast(document.querySelector('#toast')).show();
            } else {
                alert("Testcase save error — contact admin to review log");
            }
        });
});

// Convert stored UTC times to browser local time on page load.
$(document).ready(function() {
    $("#timezone").val(new Date().getTimezoneOffset());

    if ($("#time-start").val()) {
        $("#time-start").val(utcToLocal($("#time-start").val()));
    }
    if ($("#time-end").val()) {
        $("#time-end").val(utcToLocal($("#time-end").val()));
    }

    var state = $("#state").val();
    if (state === "Running" && $("#time-start").val()) {
        startElapsedTimer();
    } else if (state === "Complete" && $("#time-start").val()) {
        updateElapsedDisplay();
    }
});

// Elapsed timer.
var elapsedInterval = null;

function formatElapsed(ms) {
    var s = Math.floor(ms / 1000);
    var h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), sec = s % 60;
    return String(h).padStart(2,'0') + ':' + String(m).padStart(2,'0') + ':' + String(sec).padStart(2,'0');
}

function startElapsedTimer() {
    stopElapsedTimer();
    updateElapsedDisplay();
    elapsedInterval = setInterval(updateElapsedDisplay, 1000);
}

function stopElapsedTimer() {
    if (elapsedInterval) { clearInterval(elapsedInterval); elapsedInterval = null; }
}

function updateElapsedDisplay() {
    var startVal = $("#time-start").val();
    if (!startVal) { $("#elapsed-time").text("00:00:00"); return; }
    var end = $("#time-end").val() ? new Date($("#time-end").val()) : new Date();
    var elapsed = end - new Date(startVal);
    if (elapsed < 0) elapsed = 0;
    $("#elapsed-time").text(formatElapsed(elapsed));
}

function updateTimerBadge(state) {
    var badge = $("#elapsed-timer");
    badge.removeClass("bg-secondary bg-warning bg-primary text-dark text-white");
    if (state === "Running") { badge.addClass("bg-warning text-dark"); }
    else if (state === "Complete") { badge.addClass("bg-primary"); }
    else { badge.addClass("bg-secondary"); }
}

// Start / stop / restart timer button.
$("#run-button").click(function() {
    var now = new Date();
    now.setMinutes(now.getMinutes() - now.getTimezoneOffset());
    var ts = now.toISOString().slice(0, 16);
    var label = $("#run-button").text();

    if (label === "Start" || label === "Restart") {
        $("#time-start").val(ts);
        $("#time-end").val("");
        $("#run-button").text("Stop").removeClass("btn-outline-success btn-outline-warning").addClass("btn-outline-danger");
        $("#state").val("Running").removeClass("bg-primary text-white").addClass("bg-warning text-dark");
        updateTimerBadge("Running");
        startElapsedTimer();
    } else if (label === "Stop") {
        $("#time-end").val(ts);
        $("#run-button").text("Restart").removeClass("btn-outline-danger").addClass("btn-outline-warning");
        $("#state").val("Complete").removeClass("bg-warning text-dark").addClass("bg-primary text-white");
        updateTimerBadge("Complete");
        stopElapsedTimer();
        updateElapsedDisplay();
    }
});

// Delete evidence file via AJAX.
$(document).on("click", ".evidence-delete", function(event) {
    var target = event.target.tagName === "I" ? event.target.parentNode : event.target;
    var colour = $(target).attr("class").includes("evidence-red") ? "red" : "blue";
    var url = $(target).next("a").attr("href").split("?")[0].replace("/evidence/", "/evidence/" + colour + "/");
    apiFetch(url, 'DELETE')
        .then(function(r) {
            if (r.ok) $(target).parent().remove();
        });
});

// Inject newly uploaded evidence into the DOM after a successful save.
function displayNewEvidence(form) {
    ["red", "blue"].forEach(function(colour) {
        form.getAll(colour + "files").forEach(function(file) {
            if (!file.name) return;
            var filename = sanitiseFilename(file.name);
            var testcaseId = window.location.pathname.split("/").slice(-1)[0];
            var isImage = /\.(png|jpg|jpeg)$/i.test(file.name);
            var safeName = escAttr(filename);
            var html = '<li class="list-group-item">' +
                '<button type="button" class="btn btn-outline-danger btn-sm me-2 evidence-delete evidence-' + colour + '"><i class="bi-trash small">&zwnj;</i></button>' +
                '<a href="/testcase/' + testcaseId + '/evidence/' + safeName + '?download=true" class="btn btn-outline-primary btn-sm me-2"><i class="bi-download small">&zwnj;</i></a>';
            if (isImage) {
                html += '<a href="/testcase/' + testcaseId + '/evidence/' + safeName + '" target="_blank">' +
                    '<img class="img-fluid img-thumbnail" style="max-width:80%" src="/testcase/' + testcaseId + '/evidence/' + safeName + '"/></a>' +
                    '<input style="margin-left:6em;width:80%;" class="form-control form-control-sm" type="text" placeholder="Caption..." value="" id="' + colour.toUpperCase() + safeName + '" name="' + colour.toUpperCase() + safeName + '"/>';
            } else {
                html += '<span class="name small">' + filename + '</span>';
            }
            $('#evidence-' + colour).append(html);
            $('#' + colour + 'files').val("");
        });
    });
}

function escAttr(str) {
    return String(str).replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/'/g,'&#39;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

function sanitiseFilename(filename) {
    return filename
        .split('/').pop().split('\\').pop()
        .normalize('NFKD').replace(/[̀-ͯ]/g, '')
        .replace(/[^\w.-]/g, '_')
        .replace(/^[._-]+|[._-]+$/g, '') || 'unnamed';
}
