-- dump.lua prints, as JSON, every nvim-lspconfig config under <dir>/lsp,
-- nvim's extension and filename filetype tables, and nvim's version.
-- gen.mjs runs it:
--
--	nvim -l internal/serversgen/dump.lua <nvim-lspconfig dir>
local dir = assert(arg[1], 'usage: nvim -l dump.lua <nvim-lspconfig dir>')
vim.opt.rtp:prepend(dir)

local function has_function(v)
  if type(v) == 'function' then
    return true
  end
  if type(v) == 'table' then
    for _, x in pairs(v) do
      if has_function(x) then
        return true
      end
    end
  end
  return false
end

local kept = { cmd = true, filetypes = true, root_markers = true, init_options = true, settings = true }

-- A config runs when it loads, so a value can come out of this machine
-- (a path under $HOME, nvim's version, an unset environment variable
-- leaving a hole in cmd). Such a field is computed, like a function.
local machine = { vim.env.HOME, vim.fn.getcwd(), dir, tostring(vim.version()) }

local function machine_dependent(field, value)
  if field == 'cmd' then
    for i = 1, table.maxn(value) do
      if type(value[i]) ~= 'string' then
        return true
      end
    end
  end
  local encoded = vim.json.encode(value)
  for _, s in ipairs(machine) do
    if encoded:find(s, 1, true) then
      return true
    end
  end
  return false
end

local function load_config(file)
  local name = vim.fn.fnamemodify(file, ':t:r')
  local ok, config = pcall(dofile, file)
  if not ok or type(config) ~= 'table' then
    io.stderr:write(name .. ': ' .. tostring(config) .. '\n')
    return nil
  end
  local out = { name = name, dynamic = {} }
  for field, value in pairs(config) do
    if has_function(value) or (kept[field] and machine_dependent(field, value)) then
      table.insert(out.dynamic, field)
    elseif kept[field] then
      out[field] = value
    end
  end
  table.sort(out.dynamic)
  return out
end

-- The tables are upvalues of vim.filetype.match: nvim exposes no accessor.
local function upvalue(fn, want)
  for i = 1, math.huge do
    local name, value = debug.getupvalue(fn, i)
    if name == nil then
      error('vim.filetype.match has no upvalue ' .. want)
    elseif name == want then
      return value
    end
  end
end

-- A detector function reads the buffer, so it is asked about an empty one:
-- what nvim decides for a new file of that name. A table is nvim's
-- {ft, {priority=...}} form.
local function plain(tbl, sample)
  local out = {}
  for key, ft in pairs(tbl) do
    if type(ft) == 'table' then
      ft = ft[1]
    end
    if type(ft) == 'function' then
      local ok, got = pcall(vim.filetype.match, { filename = sample(key), contents = { '' } })
      ft = ok and got or nil
    end
    if type(ft) == 'string' then
      out[key] = ft
    end
  end
  return out
end

local configs = {}
for _, file in ipairs(vim.fn.globpath(dir .. '/lsp', '*.lua', false, true)) do
  table.insert(configs, load_config(file))
end
table.sort(configs, function(a, b)
  return a.name < b.name
end)

io.stdout:write(vim.json.encode({
  nvim = tostring(vim.version()),
  configs = configs,
  extension = plain(upvalue(vim.filetype.match, 'extension'), function(ext)
    return 'x.' .. ext
  end),
  filename = plain(upvalue(vim.filetype.match, 'filename'), function(name)
    return name
  end),
}))
