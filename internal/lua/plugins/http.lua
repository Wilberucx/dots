-- Built-in HTTP plugin for dots.
-- Provides download functionality for binary dependencies.
-- Loaded via: require("dots.http")
-- 
-- Exports:
--   http.download(url, dest) → downloads URL to destination path

local http = {}

function http.download(url, dest)
  -- Use curl or wget, whichever is available
  local handle = io.popen("command -v curl 2>/dev/null")
  local has_curl = handle:read("*a"):match("%S")
  handle:close()

  if has_curl then
    os.execute(string.format("curl -fsSL '%s' -o '%s'", url, dest))
  else
    os.execute(string.format("wget -q '%s' -O '%s'", url, dest))
  end

  -- Verify download
  local f = io.open(dest, "r")
  if f then
    f:close()
    return true
  end
  return false, "download failed: " .. url
end

return http
