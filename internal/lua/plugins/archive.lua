-- Built-in Archive plugin for dots.
-- Provides tar/zip extraction for binary dependencies.
-- Loaded via: require("dots.archive")
--
-- Exports:
--   archive.extract_tar(archive, dest, member) → extracts tar.gz to dest
--   archive.extract_zip(archive, dest) → extracts zip to dest

local archive = {}

function archive.extract_tar(archive_path, dest_dir, member)
  if member and member ~= "" then
    -- Extract specific member
    local cmd = string.format("tar -xzf '%s' -C '%s' '%s'", archive_path, dest_dir, member)
    os.execute(cmd)

    -- Move to final location if needed
    local extracted = dest_dir .. "/" .. member
    return extracted
  else
    -- Extract all
    os.execute(string.format("tar -xzf '%s' -C '%s'", archive_path, dest_dir))
    return nil
  end
end

function archive.extract_zip(archive_path, dest_dir)
  os.execute(string.format("unzip -o '%s' -d '%s'", archive_path, dest_dir))
end

return archive
