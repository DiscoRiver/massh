18/09/2021
- Updated Config.SetPublicKeyAuth to accept a key passphrase. Will break existing uses of this function. Use v1.4.0 to avoid this change temporarily.

24/07/2021
- Added JobStack to config. Multiple jobs are process automatically and do NOT reuse and active session. They return separate results, meaning you would have multiple `Result` types with the same host, but a different job string. Both `Job` and `JobStack` cannot be present when executing `Run()` or `Stream()`