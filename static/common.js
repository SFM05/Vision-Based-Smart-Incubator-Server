/* ============================================
   Shared Utilities — Smart Incubator Dashboard
   ============================================ */

var BJ_OFFSET = 8 * 3600000;

function pad2(n) {
  return String(n).padStart(2, '0');
}

function utcMsToBeijing(utcMs) {
  return new Date(utcMs + BJ_OFFSET);
}

function formatBeijingFull(utcMs) {
  var d = utcMsToBeijing(utcMs);
  return d.getUTCFullYear() + '-' +
    pad2(d.getUTCMonth() + 1) + '-' +
    pad2(d.getUTCDate()) + ' ' +
    pad2(d.getUTCHours()) + ':' +
    pad2(d.getUTCMinutes()) + ':' +
    pad2(d.getUTCSeconds());
}

function formatDatetimeLocal(date) {
  return date.getFullYear() + '-' +
    pad2(date.getMonth() + 1) + '-' +
    pad2(date.getDate()) + 'T' +
    pad2(date.getHours()) + ':' +
    pad2(date.getMinutes());
}

function queryQuick(seconds) {
  var now = Math.floor(Date.now() / 1000);
  var start = (now - seconds) * 1000000;
  var end = now * 1000000;
  document.getElementById('start-time').value = formatDatetimeLocal(new Date((now - seconds) * 1000));
  document.getElementById('end-time').value = formatDatetimeLocal(new Date(now * 1000));
  doQuery(start, end);
}

function queryCustom() {
  var startStr = document.getElementById('start-time').value;
  var endStr = document.getElementById('end-time').value;
  if (!startStr || !endStr) {
    showError('请选择起始和截止时间');
    return;
  }
  var start = new Date(startStr).getTime() * 1000;
  var end = new Date(endStr).getTime() * 1000;
  doQuery(start, end);
}

function showError(msg) {
  var el = document.getElementById('error-message');
  el.textContent = msg;
  el.style.display = 'flex';
}

function hideError() {
  document.getElementById('error-message').style.display = 'none';
}

function showInfo(msg) {
  var el = document.getElementById('info-message');
  if (!el) return;
  el.textContent = msg;
  el.style.display = 'flex';
}

function hideInfo() {
  var el = document.getElementById('info-message');
  if (el) el.style.display = 'none';
}

function initTimeDefaults() {
  var now = new Date();
  document.getElementById('end-time').value = formatDatetimeLocal(now);
  document.getElementById('start-time').value = formatDatetimeLocal(new Date(now.getTime() - 3600000));
}
