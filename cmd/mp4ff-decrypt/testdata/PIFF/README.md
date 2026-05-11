# PIFF samples

The `audio/` and `video/` content uses PIFF uuid-boxes for sample
encryption (senc) but declares the common encryption scheme as `cenc`.

The `wma_piff_scheme/` content uses the legacy PIFF protection
scheme (`schm.scheme_type = 'piff'`) with the PIFF Track Encryption
UUID box (8974dbce-7be7-4c51-84f9-7148f9882554) inside `schi`. It is
a trimmed (first fragment, pssh stripped) WMA audio sample from the
asset attached to https://github.com/Eyevinn/mp4ff/issues/496. The
KID/Key are the public values from that issue.