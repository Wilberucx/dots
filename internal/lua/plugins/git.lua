-- Built-in Git plugin for dots.
-- Provides git clone/checkout for git dependencies.
-- Loaded via: require("dots.git")
--
-- Exports:
--   git.clone(url, dest) → clones repository to destination
--   git.checkout(ref, dir) → checks out a ref in the repository

local git = {}

function git.clone(url, dest)
  os.execute(string.format("git clone '%s' '%s'", url, dest))
  local f = io.open(dest .. "/.git/HEAD", "r")
  if f then
    f:close()
    return true
  end
  return false, "git clone failed: " .. url
end

function git.checkout(ref, dir)
  os.execute(string.format("git -C '%s' checkout '%s'", dir, ref))
end

return git
