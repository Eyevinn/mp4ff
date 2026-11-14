### Fixed

- `InitProtectData.NewFragmentEncryptor`, `EncryptFragment`, `EncryptFragments` and `mp4ff-encrypt` reject
  cbcs encryption when the IV differs from the init's `tenc` constant IV or the init has no constant IV.
