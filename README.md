# Replay
Record & Replay traffic - alternative to dependency injection.

# Language agnostic

```
cd app/redis
docker compose up -d
```

Now play with `redis-cli` or point your app to `:6379`.


# TODO

* ~~Support Redis~~
* ~~Support Postgres~~
* Demo app calling Postgres and uses Redis as cache;
* Support generic TCP, timeout based request detection on replay, switch of direction to detect request on record, also see via;
* Ready to deploy containers for record/replay;
* Support HTTP;
* Add execution layer for running tests, like controller that can __execute__ recordings to trigger tests, without having to implement them in test;
* Support gRPC;
* Examples with popular APIs like S3 or Stripe;
* Address race issues after eliminiating potential problems:
    * Do NOT reuse containers between test runs;
    * Do NOT write to disk from writer, just compare to golden files;
    * Check order of all `Close()` implementations;
    * Check order of terminations in tests;
    * Examine use of channels, make sure they are closed in logical order, e.g. closed channel is not required if connections are closed properly;
