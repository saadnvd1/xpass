# Merge by entry on pull

The vault is one encrypted file; git cannot merge it and the web vault cannot
either (it never has the password). So `xpass pull` decrypts base, local and
remote and merges by entry ID: one side changed → that side; both → higher
Version, then later UpdatedAt (reported as a conflict); deleted on one side and
untouched on the other → deleted; deleted vs edited → the edit. A remote that
does not decrypt, or that is an exact older copy, is refused.
