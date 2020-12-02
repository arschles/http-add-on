# Communicating the Queue Size between the interceptor and scaler

- Split communication between "live" updates and caching
- Live updating happens by one way communication between interceptor => scaler
- Interceptor pushes data over {NATS, Redis, others?} (pluggable), data is not persisted anywhere
  - Data format: `{last_queue_size: 123, current_queue_size: 234, timestamp: 11:56, interceptor_uuid: asdfpoishdp;fgkjsdfng}`
    - also, a tombstone so an interceptor can indicate to a scaler it's going away: `{leaving: true, interceptor_uuid: asdfgasdfasdfa}` - this helps the scaler avoid doing a health check on a bucket that no longer exists
- Scaler accepts data push from any interceptor and:
  - Creates bucket for that interceptor based on uuid if necessary
  - calculates delta and updates the bucket with:
    - the new total number of pending requests for that bucket
    - the timestamp of when it got this message
- In the background, scaler:
  - does a "health check" on each bucket. if it hasn't gotten new data in X "ticks" (interceptor and scaler need to agree on this ticks value before runtime), zero it out (or delete it, not sure if there's any difference?)
  - checkpoint the buckets data - write it to a ConfigMap to cache it
    - If the scaler crashes, it should look for it in the cache, restore it, and start from there - give KEDA back all the same data when it asks for it
      - The health check behavior can stay approximately the same. it should immediately start the ticker, but do the initial health check after X * (small_delay) ticks - this is to help avoid some "flapping"
      - The checkpoint behavior stays the same
