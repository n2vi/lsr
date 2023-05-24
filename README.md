# lsr
Command lsr prints a recursive listing of the current directory, including
quoted filename, size, modtime, and sha256(contents).

Main output goes to local file ".lsr", which is created 0600 if it doesn't exist.
Diagnostic output goes to Stdout as lines of the form: status filename
where status is one of
- N new
- D deleted
- M modified (size or hash changed, and mtime advanced)
- R reverted (size or hash changed, and mtime went backwards)
- T touched (mtime changed but hash did not)
- C corrupted (size or hash changed but mtime did not)
or silent for files that are same as before.

This is not a security tool. Anyone that can maliciously change a file can
just as well change .lsr. But it is a good way to review what you've worked
on in the recent past or catch unintended changes.

Lsr is a minor variation of a command I've been using since the early '80s
to organize backups, mirror projects, and check for filesystem corruption.
Its use in administering netlib is described in ACM TOMS (1995) 21:1:89-97.
I created it at Bell Labs when the Interdata hardware (first Unix port)
was silently corrupting files. Given recent stories about declining
reliability of consumer SSD, it may be prudent for a few people like
me to resume such checking.

