# Replay
Record & Replay traffic - alternative to dependency injection.


# TODO

* ~~Support Redis~~
* ~~Support Postgres~~
* Support HTTP
* Support gRPC
* Address race issues after eliminiating potential problems:
    * Do NOT reuse containers between test runs;
    * Do NOT write to disk from writer, just compare to golden files;
    * Check order of all `Close()` implementations;
    * Check order of terminations in tests;
    * Examine use of channels, make sure they are closed in logical order, e.g. closed channel is not required if connections are closed properly;
