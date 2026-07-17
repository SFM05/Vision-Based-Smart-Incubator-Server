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
  var span = el.querySelector('span');
  if (span) span.textContent = msg;
  else el.textContent = msg;
  el.style.display = 'flex';
}

function hideError() {
  document.getElementById('error-message').style.display = 'none';
}

function showInfo(msg) {
  var el = document.getElementById('info-message');
  if (!el) return;
  var span = el.querySelector('span');
  if (span) span.textContent = msg;
  else el.textContent = msg;
  el.style.display = 'flex';
}

function hideInfo() {
  var el = document.getElementById('info-message');
  if (el) el.style.display = 'none';
}

function setDeviceStatus(message, isError) {
  var el = document.getElementById('device-status');
  if (!el) return;
  el.innerHTML = '';
  el.appendChild(document.createTextNode(message || ''));
  el.classList.toggle('error', !!isError);
}

function setDeviceStatusWithManualAction(message, isError) {
  setDeviceStatus(message + ' ', isError);
  var el = document.getElementById('device-status');
  var uuidEl = document.getElementById('uuid');
  if (!el || !uuidEl || uuidEl.tagName !== 'SELECT') return;

  var button = document.createElement('button');
  button.type = 'button';
  button.className = 'selector-manual-btn';
  button.textContent = '手动输入';
  button.addEventListener('click', function() {
    enableManualDeviceFallback('已选择手动输入');
  });
  el.appendChild(button);
}

function setDeviceStatusWithSelectAction(message, isError) {
  setDeviceStatus(message + ' ', isError);
  var el = document.getElementById('device-status');
  var uuidEl = document.getElementById('uuid');
  if (!el || !uuidEl || uuidEl.tagName !== 'INPUT') return;

  var button = document.createElement('button');
  button.type = 'button';
  button.className = 'selector-manual-btn';
  button.textContent = '使用列表';
  button.addEventListener('click', function() {
    enableDeviceSelectMode();
  });
  el.appendChild(button);
}

function resetSelect(select, message) {
  select.innerHTML = '';
  var option = document.createElement('option');
  option.value = '';
  option.textContent = message;
  select.appendChild(option);
}

function replaceElement(oldEl, newEl) {
  oldEl.parentNode.replaceChild(newEl, oldEl);
  return newEl;
}

function enableDeviceSelectMode() {
  var uuidInput = document.getElementById('uuid');
  if (!uuidInput || uuidInput.tagName !== 'INPUT') return;

  localStorage.setItem('incubator_uuid', uuidInput.value.trim());

  var uuidSelect = document.createElement('select');
  uuidSelect.id = 'uuid';
  uuidSelect.className = uuidInput.className;
  uuidSelect.disabled = true;
  resetSelect(uuidSelect, '正在加载设备...');
  replaceElement(uuidInput, uuidSelect);

  var plateInput = document.getElementById('plateid');
  if (plateInput && plateInput.tagName === 'INPUT') {
    localStorage.setItem('incubator_plateid', plateInput.value.trim());
    var plateSelect = document.createElement('select');
    plateSelect.id = 'plateid';
    plateSelect.disabled = true;
    resetSelect(plateSelect, '请先选择设备');
    replaceElement(plateInput, plateSelect);
  }

  initDeviceSelectors();
}

function enableManualDeviceFallback(message) {
  var uuidSelect = document.getElementById('uuid');
  if (!uuidSelect || uuidSelect.tagName !== 'SELECT') return;

  var uuidInput = document.createElement('input');
  uuidInput.id = 'uuid';
  uuidInput.type = 'text';
  uuidInput.placeholder = '请输入设备 UUID';
  uuidInput.className = uuidSelect.className;
  uuidInput.value = localStorage.getItem('incubator_uuid') || '';
  replaceElement(uuidSelect, uuidInput);
  uuidInput.addEventListener('input', function() {
    localStorage.setItem('incubator_uuid', uuidInput.value.trim());
  });

  var plateSelect = document.getElementById('plateid');
  if (plateSelect && plateSelect.tagName === 'SELECT') {
    var plateInput = document.createElement('input');
    plateInput.id = 'plateid';
    plateInput.type = 'number';
    plateInput.min = '0';
    plateInput.step = '1';
    plateInput.required = true;
    plateInput.inputMode = 'numeric';
    plateInput.value = localStorage.getItem('incubator_plateid') || '0';
    plateInput.oninput = validatePlateIDInput;
    replaceElement(plateSelect, plateInput);
    plateInput.addEventListener('input', function() {
      localStorage.setItem('incubator_plateid', plateInput.value.trim());
    });
  }

  setDeviceStatusWithSelectAction(message + ' 已切换为手动输入。', true);
}

async function readJSONResponse(resp) {
  var text = await resp.text();
  try {
    return JSON.parse(text);
  } catch (err) {
    var snippet = text.trim().slice(0, 80) || '空响应';
    throw new Error('接口返回非 JSON（HTTP ' + resp.status + '）：' + snippet);
  }
}

function populatePlateSelect(devicesByUUID, uuid) {
  var plateSelect = document.getElementById('plateid');
  if (!plateSelect || plateSelect.tagName !== 'SELECT') return;

  var savedPlate = localStorage.getItem('incubator_plateid') || '';
  var device = devicesByUUID[uuid];
  var plates = device ? device.plates || [] : [];

  resetSelect(plateSelect, plates.length ? '请选择盘位号' : '该设备暂无盘位');
  for (var i = 0; i < plates.length; i++) {
    var option = document.createElement('option');
    option.value = String(plates[i]);
    option.textContent = '盘位 ' + plates[i];
    plateSelect.appendChild(option);
  }

  if (savedPlate && plates.indexOf(Number(savedPlate)) !== -1) {
    plateSelect.value = savedPlate;
  } else if (plates.length > 0) {
    plateSelect.value = String(plates[0]);
  }

  plateSelect.disabled = plates.length === 0;
  if (plateSelect.value) {
    localStorage.setItem('incubator_plateid', plateSelect.value);
  }
}

async function initDeviceSelectors() {
  var uuidSelect = document.getElementById('uuid');
  if (!uuidSelect || uuidSelect.tagName !== 'SELECT') return;

  var plateSelect = document.getElementById('plateid');
  resetSelect(uuidSelect, '正在加载设备...');
  uuidSelect.disabled = true;
  if (plateSelect) {
    resetSelect(plateSelect, '请先选择设备');
    plateSelect.disabled = true;
  }
  setDeviceStatusWithManualAction('正在从阿里云 Tablestore 获取设备列表。', false);

  try {
    var resp = await fetch('/api/devices');
    var data = await readJSONResponse(resp);
    if (!data.success) {
      throw new Error(data.message || '设备列表加载失败');
    }

    uuidSelect = document.getElementById('uuid');
    if (!uuidSelect || uuidSelect.tagName !== 'SELECT') return;
    plateSelect = document.getElementById('plateid');

    var allDevices = data.devices || [];
    var devices = plateSelect ? allDevices.filter(function(device) {
      return device.plates && device.plates.length > 0;
    }) : allDevices;

    if (devices.length === 0) {
      enableManualDeviceFallback('未发现可选设备，请确认 Tablestore 中已有数据。');
      return;
    }

    resetSelect(uuidSelect, '请选择设备 UUID');
    var devicesByUUID = {};
    for (var i = 0; i < devices.length; i++) {
      var device = devices[i];
      devicesByUUID[device.uuid] = device;
      var option = document.createElement('option');
      option.value = device.uuid;
      option.textContent = device.uuid;
      if (plateSelect) {
        option.textContent += ' (' + (device.plates || []).length + ' 个盘位)';
      }
      uuidSelect.appendChild(option);
    }

    var savedUUID = localStorage.getItem('incubator_uuid') || '';
    if (savedUUID && devicesByUUID[savedUUID]) {
      uuidSelect.value = savedUUID;
    } else {
      uuidSelect.value = devices[0].uuid;
    }
    uuidSelect.disabled = false;
    localStorage.setItem('incubator_uuid', uuidSelect.value);

    if (plateSelect) {
      populatePlateSelect(devicesByUUID, uuidSelect.value);
      plateSelect.addEventListener('change', function() {
        localStorage.setItem('incubator_plateid', plateSelect.value);
      });
    }

    uuidSelect.addEventListener('change', function() {
      localStorage.setItem('incubator_uuid', uuidSelect.value);
      if (plateSelect) {
        populatePlateSelect(devicesByUUID, uuidSelect.value);
      }
    });

    setDeviceStatusWithManualAction('设备列表已加载。', false);
  } catch (err) {
    enableManualDeviceFallback('设备列表加载失败：' + err.message);
  }
}

function initTimeDefaults() {
  var now = new Date();
  document.getElementById('end-time').value = formatDatetimeLocal(now);
  document.getElementById('start-time').value = formatDatetimeLocal(new Date(now.getTime() - 3600000));
  initDeviceSelectors();
}
