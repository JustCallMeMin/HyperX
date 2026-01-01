# Content Publish → Access Enforcement Flow

**Scope:** Cross-contract content delivery flow  
**Trigger:** Creator publishes content (post, video, file, etc.)  
**Authority:** Creator-initiated, system-enforced access control  
**Criticality:** SEV-2 (affects value delivery + creator trust, not money directly)

## Flow Sequence
```
Creator action: Publish content
  ↓
[Content] validate content:
  ├─ Check content type allowed for creator tier
  ├─ Validate file size/format
  ├─ Run moderation filters (profanity, NSFW detection)
  └─ If validation fails → reject with error
  ↓
[Content] create content record:
  ├─ content_id (UUID)
  ├─ creator_id
  ├─ content_type (post, video, audio, file, poll, etc.)
  ├─ visibility_policy (public, tier_gated, specific_tiers)
  ├─ required_tier_ids (array, if tier_gated)
  ├─ published_at (UTC timestamp)
  └─ status: "published"
  ↓
[Content] determine visibility:
  ├─ If public → accessible to everyone (logged in or not)
  ├─ If tier_gated → accessible only to subscribers of required_tier_ids
  └─ If specific_tiers → accessible to specific tier list
  ↓
[Subscription] (async) calculate eligible supporters:
  ├─ Query subscriptions with state="active" 
  ├─ AND subscription.tier_id IN required_tier_ids
  ├─ AND subscription.period_end >= now (access not expired)
  └─ Return list of supporter_ids with access
  ↓
[Content] cache access control list (ACL):
  ├─ Store content_id → [supporter_ids] mapping
  ├─ TTL: 5 minutes (balance freshness vs load)
  └─ Invalidate on subscription state change
  ↓
[Feed] (async) add to feed:
  ├─ Public feed (if public content)
  ├─ Creator feed (all supporters see in their feed)
  └─ Personalized feeds (rank by supporter preferences)
  ↓
[Notification] (async) send notifications:
  ├─ If tier_gated → notify eligible supporters only
  ├─ If public → notify all followers (or none, based on creator settings)
  ├─ Delivery methods: push notification, email digest, in-app
  └─ Batch notifications (don't send 10k emails immediately)
  ↓
[Reporting] (async) update creator metrics:
  ├─ Content published count
  ├─ Public vs gated content ratio
  └─ Average posts per week (engagement metric)
  ↓
--- ACCESS CHECK (when supporter requests content) ---
  ↓
[Content] receive access request:
  ├─ Input: content_id, supporter_id (from auth token)
  └─ Question: "Can this supporter access this content?"
  ↓
[Content] check visibility policy:
  ├─ If public → GRANT access immediately
  ├─ If tier_gated → proceed to subscription check
  └─ If specific_tiers → proceed to subscription check
  ↓
[Content] check ACL cache:
  ├─ Cache hit → return cached result (supporter_id in ACL?)
  └─ Cache miss → proceed to real-time check
  ↓
[Subscription] real-time access verification:
  ├─ Query: subscription where supporter_id AND creator_id AND state="active"
  ├─ Check: subscription.tier_id IN content.required_tier_ids
  ├─ Check: subscription.period_end >= now (access not expired)
  └─ Return: access_granted (true/false)
  ↓
[Content] enforce access decision:
  ├─ If access_granted → serve content (full post, video stream, file download)
  ├─ If access_denied → return 403 with upgrade prompt
  └─ Log access attempt (for analytics)
  ↓
[Reporting] (async) track engagement:
  ├─ Content view count
  ├─ Engagement rate (views / eligible supporters)
  └─ Popular content (for creator insights)
```

## Authority Guarantees
- **content_distribution_contract:** sole owner of content records and visibility policies
- **content_distribution_contract:** enforces access decisions (serve or deny)
- **subscription_contract:** source of truth for active subscriptions and tier access
- **feed_discovery_contract:** displays content, does NOT enforce access (display ≠ access)
- **notification_contract:** notifies supporters, does NOT grant access
- **creator_reporting_contract:** tracks metrics, does NOT affect access decisions

## Content Visibility Policies

### 1. Public Content
- **Definition:** Accessible to anyone (logged in or not)
- **Use cases:** Promotional posts, free samples, announcements
- **Access check:** Always returns true
- **Feed visibility:** Appears in public creator feed, shareable links work
- **Notification:** Sent to all followers (or none, based on creator settings)

### 2. Tier-Gated Content (Single Tier)
- **Definition:** Accessible only to subscribers of a specific tier (e.g., "Gold Tier")
- **Use cases:** Exclusive content for specific membership level
- **Access check:** 
  - Supporter must have active subscription
  - subscription.tier_id must match content.required_tier_id
  - subscription.period_end must be >= now
- **Feed visibility:** Appears in feed for eligible supporters only
- **Notification:** Sent to eligible tier subscribers only

### 3. Tier-Gated Content (Multiple Tiers)
- **Definition:** Accessible to subscribers of any tier in a list (e.g., "Gold or Platinum")
- **Use cases:** Premium content accessible to multiple tier levels
- **Access check:** subscription.tier_id IN content.required_tier_ids
- **Feed visibility:** Appears in feed for any eligible tier
- **Notification:** Sent to all eligible tier subscribers

### 4. Tier-Gated Content (Hierarchical Access)
- **Definition:** Higher tiers automatically access lower tier content
- **Example:** Platinum tier can access Gold tier content
- **Implementation:** 
  - Tiers have tier_level field (e.g., Bronze=1, Silver=2, Gold=3, Platinum=4)
  - Access granted if subscription.tier.tier_level >= content.required_tier.tier_level
- **Access check:** Compare tier levels, not just IDs
- **Feed visibility:** Show content to supporter's tier level and above

### 5. Supporter-Specific Content (rare)
- **Definition:** Accessible only to specific supporter_ids (e.g., custom commissions)
- **Use cases:** Personalized content, 1-on-1 interactions
- **Access check:** supporter_id IN content.allowed_supporter_ids
- **Feed visibility:** Only visible to specified supporters
- **Notification:** Sent only to specified supporters

### 6. Scheduled Content (future publish)
- **Definition:** Content created but publish_at is in the future
- **Use cases:** Scheduled posts, drip content campaigns
- **Access check:** if now < content.publish_at → deny access (even to creator)
- **Feed visibility:** Not visible until publish_at
- **Background job:** Cron job runs every minute, publishes content where publish_at <= now

## Idempotency Strategy

### Content Creation Level
- **Key:** f"content_publish:{creator_id}:{content_hash}:{publish_timestamp}"
- **Storage:** content table with UNIQUE constraint on (creator_id, content_hash, published_at)
- **Protection:** Prevents duplicate content if creator clicks "Publish" multiple times
- **Note:** content_hash = hash(title + body + attachments), detects exact duplicates

### ACL Cache Level
- **Key:** f"content_acl:{content_id}"
- **Storage:** Redis cache with 5-minute TTL
- **Protection:** Prevents redundant subscription queries on high-traffic content
- **Invalidation:** On subscription state change (active→cancelled, tier change)

### Feed Addition Level
- **Key:** f"feed_item:{feed_type}:{content_id}"
- **Storage:** feed_items table with UNIQUE constraint on (feed_type, content_id)
- **Protection:** Prevents duplicate feed entries if job retries
- **Check:** if feed_item exists → skip insert (idempotent)

### Notification Level
- **Key:** f"content_notification:{content_id}:{supporter_id}"
- **Storage:** sent_notifications table
- **Protection:** Prevents duplicate "new content" notifications
- **Batch handling:** Notifications sent in batches of 100, each batch has own idempotency key

### Access Log Level
- **Key:** f"access_log:{content_id}:{supporter_id}:{date}"
- **Storage:** access_logs table (for analytics)
- **Protection:** One access log entry per supporter per content per day (deduplicate views)
- **Aggregation:** View count = COUNT(DISTINCT supporter_id) per content_id

## Error Handling

| Failure Point | Root Cause | Compensation Strategy | Criticality |
|--------------|------------|----------------------|-------------|
| Content validation fails | Invalid file type, size exceeds limit, moderation flag | Return 400 error to creator, explain validation failure | SEV-3 |
| Content creation fails | DB timeout, constraint violation | Retry 3x, then return 500 error to creator | SEV-2 |
| ACL calculation fails | Subscription service down, query timeout | Fallback: real-time access check on every request (slower but correct) | SEV-2 |
| Feed addition fails | Feed service down, queue full | Async retry (indefinite), eventual consistency acceptable | SEV-3 |
| Notification fails | Email provider down, push notification service down | Queue retry (24hr window), acceptable to miss some notifications | SEV-3 |
| Access check fails (request-time) | Subscription service down, cache unavailable | DENY access (fail closed), return 503 error to supporter | SEV-1 |
| Content delivery fails | CDN down, file storage unavailable | Return 503 error, alert ops immediately | SEV-1 |

### Critical Rules
- **Content creation MUST succeed** before any downstream effects (feed, notification)
- **Access check failure MUST default to DENY** (fail closed, not open)
- **If subscription service down during access check** → deny access, don't serve content to unpaid users
- **Feed/notification failures are acceptable** (eventual consistency, not critical path)
- **Content delivery failure (CDN down) is SEV-1** (supporter paid for access, can't view)

### Access Check Fallback Strategy
- **Primary:** Check ACL cache (5min TTL)
- **Fallback 1:** Real-time subscription query (if cache miss)
- **Fallback 2:** Check access_entitlements table (if subscription service down)
- **Fallback 3:** DENY access and return 503 error (if all services down)
- **Never:** Default to GRANT access (security-critical)

## Observability Requirements

### Events Emitted (in sequence)

1. **ContentPublished** (from content)
   - payload: {content_id, creator_id, content_type, visibility_policy, required_tier_ids, published_at, file_size?, moderation_score?}

2. **ContentACLCalculated** (from content, after ACL cache built)
   - payload: {content_id, eligible_supporter_count, acl_cache_key, cache_ttl}

3. **ContentAddedToFeed** (from feed, async)
   - payload: {content_id, feed_type: "public" | "creator" | "personalized", added_at}

4. **ContentNotificationSent** (from notification, async, batch events)
   - payload: {content_id, notification_batch_id, recipient_count, sent_at}

5. **ContentAccessGranted** (from content, per access request)
   - payload: {content_id, supporter_id, access_granted: true, access_method: "cache" | "realtime" | "fallback", response_time_ms}

6. **ContentAccessDenied** (from content, per access request)
   - payload: {content_id, supporter_id, access_granted: false, denial_reason: "no_subscription" | "wrong_tier" | "expired" | "service_unavailable"}

7. **ContentEngagementTracked** (from reporting, aggregated)
   - payload: {content_id, view_count, unique_viewers, engagement_rate, tracked_at}

### Structured Logging
Each step logs:
- **correlation_id** (same across publish flow)
- **content_id**
- **creator_id**
- **supporter_id** (if access check)
- **content_type** (post, video, file, etc.)
- **visibility_policy**
- **required_tier_ids** (array)
- **access_granted** (true/false, if access check)
- **denial_reason** (if access denied)
- **access_check_duration_ms** (performance metric)
- **step_name**
- **step_status** (success | failed | skipped)
- **timestamp**

### Metrics (for alerting)

#### Real-time Metrics
- **content_publish_duration_seconds** (p50, p95, p99)
  - Alert if p99 > 10 seconds (creator experience degraded)
  
- **access_check_duration_ms** (p50, p95, p99)
  - Alert if p99 > 200ms (supporter experience degraded)
  
- **access_denial_rate** (denials / total access requests)
  - Baseline: expect 5-10% (legitimate denials for non-subscribers)
  - Alert if > 20% (indicates subscription service issues or ACL cache problems)
  
- **acl_cache_hit_rate** (cache hits / total access checks)
  - Target: > 90% (cache working well)
  - Alert if < 70% (cache TTL too short or invalidation too aggressive)
  
- **content_delivery_error_rate** (503 errors / total requests)
  - Alert if > 0.1% (CDN or storage issues)

#### Business Metrics
- **content_published_count** (per creator, daily)
  - Track creator activity levels
  
- **public_vs_gated_ratio** (public content / total content, per creator)
  - Insight: creators with more public content may have better discovery
  
- **average_view_count_per_content** (views / content, per creator)
  - Track engagement health
  
- **engagement_rate** (unique viewers / eligible subscribers, per content)
  - Baseline: expect 30-50% for active communities
  - Alert creator if < 10% (content not resonating)
  
- **content_type_distribution** (posts vs videos vs files, per creator)
  - Product insight: which content types most popular

#### Performance Metrics
- **subscription_query_duration_ms** (during access check)
  - Alert if p99 > 100ms (DB performance issue)
  
- **acl_cache_build_duration_ms** (after content publish)
  - Alert if p99 > 2 seconds (subscription query too slow)
  
- **cdn_response_time_ms** (content delivery latency)
  - Alert if p99 > 1 second (CDN performance issue)

### Audit Trail

#### Content Table
- content_id, creator_id, content_type, visibility_policy, required_tier_ids
- title, body, attachments (file URLs), file_size
- published_at, updated_at, status (draft | published | deleted | flagged)
- moderation_score, moderation_flags (array)

#### Access Logs (for analytics)
- access_log_id, content_id, supporter_id, accessed_at
- access_granted (true/false), denial_reason
- access_method (cache | realtime | fallback)
- response_time_ms

#### Feed Items (denormalized for fast queries)
- feed_item_id, feed_type, content_id, creator_id
- added_at, rank (for sorting)
- Soft delete: removed_at (if content deleted)

#### Content Moderation Log
- moderation_id, content_id, moderation_type: "automated" | "manual"
- flagged_at, flag_reason, reviewed_by (admin_user_id if manual)
- action_taken: "approved" | "removed" | "age_restricted"

## Human Override Points

### Ops Can Intervene At:

#### 1. **Force content visibility change**
- **Use case:** Content accidentally published as public, should be tier-gated
- **Action:** Update content.visibility_policy and required_tier_ids
- **Authority:** Admin with "content_moderation" permission
- **Side effects:** 
  - Invalidate ACL cache immediately
  - Remove from public feed if changed to gated
  - Notify supporters who lost access (if changing from public to gated)
- **Audit:** admin_user_id, change_reason, old_policy, new_policy

#### 2. **Grant temporary access to specific supporter**
- **Use case:** Supporter subscription expired but creator wants to grant access for specific content
- **Action:** Add supporter_id to content.allowed_supporter_ids override list
- **Authority:** Admin with "access_override" permission OR creator themselves
- **Side effects:** 
  - Supporter can access content even without active subscription
  - Access grant expires after 30 days (configurable)
- **Audit:** granter_user_id (admin or creator), grant_reason, expiration_date

#### 3. **Flag content for moderation**
- **Use case:** Content violates TOS, requires review
- **Action:** Update content.status = "flagged", prevent access until reviewed
- **Authority:** Admin with "content_moderation" permission OR automated moderation system
- **Side effects:** 
  - Content removed from all feeds immediately
  - Access denied to all supporters (even with subscription)
  - Notification sent to creator about flagging
- **Audit:** flagged_by (admin_user_id | "automated"), flag_reason, flagged_at

#### 4. **Restore deleted content**
- **Use case:** Creator accidentally deleted content, wants to restore
- **Action:** Update content.status from "deleted" to "published"
- **Authority:** Admin with "content_restoration" permission OR creator themselves (within 30 days)
- **Side effects:** 
  - Content reappears in feeds
  - Notifications NOT resent (avoid spam)
  - Access checks resume normally
- **Audit:** restored_by, restore_reason, original_deletion_date

#### 5. **Manually trigger notification for content**
- **Use case:** Notification job failed, supporters didn't get notified about new content
- **Action:** Re-queue notification job for specific content_id
- **Authority:** Admin with "notification_retry" permission
- **Validation:** Check sent_notifications table to avoid duplicate sends
- **Side effects:** Notifications sent to eligible supporters (deduplicated)
- **Audit:** admin_user_id, retry_reason, original_publish_date

### Audit Requirements
- All manual actions logged in content.admin_actions table
- Required fields: admin_user_id, action_type, reason (min 20 chars), timestamp
- For moderation actions: flag_reason (predefined list) + custom notes
- Immutable: cannot delete content records (soft delete only)
- Retention: 3 years (TOS compliance)

## Replay Safety

### Content Publish Replay
- **Scenario:** Creator clicks "Publish" button multiple times
- **Safety:** content_hash deduplication (UNIQUE constraint on creator_id + content_hash + published_at)
- **Result:** Second publish attempt fails with "Content already exists" error

### ACL Cache Rebuild Replay
- **Scenario:** ACL cache build job runs twice due to retry
- **Safety:** Cache SET operation is idempotent (same key, same value)
- **Result:** Cache updated twice, no harm (same supporter list)

### Feed Addition Replay
- **Scenario:** Feed job retries after timeout
- **Safety:** UNIQUE constraint on (feed_type, content_id)
- **Result:** Second insert fails with SQL error, safely ignored

### Notification Replay
- **Scenario:** Notification batch job retries
- **Safety:** Idempotency key per notification recipient
- **Result:** No duplicate notifications sent

### Access Log Replay
- **Scenario:** Access request logged twice
- **Safety:** UNIQUE constraint on (content_id, supporter_id, date)
- **Result:** View count not double-counted (daily deduplication)

## Edge Cases & Business Rules

### Content Type Restrictions
- **Policy:** Different tiers may allow different content types
  - Example: Basic tier = text posts only, Premium tier = text + video
- **Enforcement:** Check creator.tier.allowed_content_types before publish
- **If violated:** Return 403 error "Video uploads require Premium tier"

### File Size Limits
- **Free tier creators:** 10 MB per file
- **Paid tier creators:** 100 MB per file
- **Enterprise creators:** 1 GB per file
- **Enforcement:** Validate file_size before upload (client-side) and after (server-side)
- **If exceeded:** Return 413 error "File too large for your tier"

### Content Moderation Automation
- **Text analysis:** Profanity filter, spam detection, keyword blacklist
- **Image analysis:** NSFW detection (e.g., AWS Rekognition, Google Cloud Vision)
- **Video analysis:** First frame thumbnail check (same as image analysis)
- **Thresholds:**
  - moderation_score > 0.8 → auto-flag, require manual review
  - moderation_score 0.5-0.8 → publish with warning
  - moderation_score < 0.5 → publish normally
- **Manual review:** Admin reviews flagged content within 24 hours

### Access During Subscription Transition
- **Scenario:** Supporter downgrades tier mid-cycle, content published for higher tier
- **Policy:** Supporter retains access until period_end (paid for full period)
- **Access check:** Compare subscription.tier_id at time of payment, not current tier
- **Implementation:** Store subscription_snapshot at period_start, use for access checks

### Access After Subscription Cancellation
- **Scenario:** Supporter cancels subscription (cancel_at_period_end = true), content published
- **Policy:** Supporter retains access until period_end
- **Access check:** subscription.status = "active_until_cancel" AND subscription.period_end >= now → GRANT
- **After period_end:** Access denied

### Content Visibility Inheritance (hierarchical tiers)
- **Example:** Creator has Bronze ($5), Silver ($10), Gold ($20) tiers
- **Policy:** Gold tier can access Silver and Bronze content, Silver can access Bronze, Bronze only Bronze
- **Implementation:** Tiers have tier_level field, access granted if supporter.tier_level >= content.required_tier_level
- **Access check:** Compare levels, not IDs

### Scheduled Content Auto-Publish
- **Background job:** Cron runs every 1 minute
- **Query:** SELECT content WHERE status="scheduled" AND publish_at <= now
- **Action:** Update status to "published", trigger full publish flow (ACL, feed, notification)
- **Idempotency:** Check status again before update (prevent race condition)

### Content Deletion Policy
- **Soft delete:** content.status = "deleted", content not served
- **Hard delete:** After 30 days, permanently delete files from storage
- **Access impact:** Access denied immediately on soft delete
- **Feed impact:** Removed from all feeds immediately
- **Supporter impact:** If supporter saved link, link returns 404 after deletion

### Content Update (Edit After Publish)
- **Policy:** Creators can edit content after publish
- **Visibility:** Edited content maintains same visibility_policy
- **Access:** No access check change (still based on tiers)
- **Notification:** Optional "content updated" notification (creator settings)
- **Audit trail:** Store content_versions table with full edit history

### Multi-File Content (e.g., Photo Gallery)
- **Structure:** One content record, multiple attachments (array of file URLs)
- **Access check:** Single access check grants access to all files in content
- **File delivery:** Each file URL signed with short-lived token (1 hour expiry)
- **If supporter shares URL:** URL expires after 1 hour, must re-authenticate

## Integration Points

### File Storage (S3, GCS, etc.)
- **Upload flow:** Creator uploads file → presigned URL → storage bucket
- **File URL:** Stored in content.attachments array
- **Access control:** Bucket is private, files served via signed URLs (generated at access time)
- **CDN:** CloudFront or Cloud CDN in front of storage for performance
- **Expiry:** Signed URLs expire after 1 hour (supporter must refresh if still viewing)

### Content Moderation Service
- **Text moderation:** Internal keyword filter + ML-based profanity detection
- **Image moderation:** AWS Rekognition DetectModerationLabels API
- **Video moderation:** Extract first frame + middle frame, run image moderation
- **API calls:** Async (don't block publish), results stored in content.moderation_score
- **Cost:** ~$0.001 per image, budget for moderation at scale

### CDN (Content Delivery Network)
- **Provider:** CloudFront, Cloudflare, Fastly
- **Caching:** Cache content files (images, videos) at edge
- **Cache invalidation:** On content deletion or visibility change
- **Signed URLs:** CDN validates signed URLs before serving (prevent hotlinking)
- **Analytics:** CDN logs for bandwidth tracking (cost allocation per creator)

### Feed Discovery Service
- **API:** POST /feeds/{feed_type}/items with content_id, creator_id, published_at
- **Ranking:** Feed service handles ranking (chronological, algorithmic, etc.)
- **This flow:** Only adds content to feed, doesn't control ranking
- **Separation:** Feed displays content, Content enforces access (clear boundary)

### Notification Service
- **API:** POST /notifications/batch with content_id, recipient_list
- **Delivery:** Email digest, push notification, in-app notification
- **Batching:** Group notifications into batches of 100 recipients (avoid rate limits)
- **Throttling:** Max 10,000 notifications per minute (platform-wide)
- **Deduplication:** Notification service handles deduplication (idempotency key per recipient)

## Performance & Scale Considerations

### Database Load
- **Content table:** Indexed on (creator_id, published_at), (content_id, status)
- **Access logs:** Partitioned by month, indexed on (content_id, accessed_at)
- **Expected load:** 50,000 content publishes/day at scale, 10M access checks/day

### Access Check Optimization
- **ACL cache hit rate target:** > 90%
- **Cache TTL:** 5 minutes (balance freshness vs query load)
- **Cache invalidation:** On subscription state change (event-driven)
- **Fallback performance:** Real-time subscription query < 100ms p99

### File Upload Performance
- **Presigned URLs:** Generated by backend, client uploads directly to S3 (no proxy)
- **Upload limits:** Max 1 GB per file, 10 files per content
- **Progress tracking:** Client-side (multipart upload with progress events)
- **Upload timeout:** 30 minutes (for large video files)

### Content Delivery Performance
- **CDN cache hit rate target:** > 95% (most requests served from edge)
- **Signed URL generation:** < 50ms p99 (in-memory signing, no DB query)
- **First-byte latency:** < 200ms p99 (CDN to supporter)
- **Video streaming:** Adaptive bitrate (HLS or DASH), generated on upload

### Notification Batching
- **Batch size:** 100 recipients per batch job
- **Concurrency:** 10 concurrent batch jobs (max 1,000 notifications/sec)
- **Expected load:** Content with 10,000 subscribers → 100 batches → ~10 seconds to complete
- **Acceptable delay:** Notifications within 5 minutes of publish (not real-time)

### Database Query Optimization
- **ACL calculation query:**
```sql
  SELECT supporter_id 
  FROM subscriptions 
  WHERE creator_id = ? 
    AND tier_id IN (?) 
    AND state = 'active' 
    AND period_end >= NOW()
```
  - Index on (creator_id, tier_id, state, period_end)
  - Expected: < 100ms for creators with 10,000 subscribers
  
- **Access check query (cache miss):**
```sql
  SELECT 1 
  FROM subscriptions 
  WHERE supporter_id = ? 
    AND creator_id = ? 
    AND tier_id IN (?) 
    AND state = 'active' 
    AND period_end >= NOW() 
  LIMIT 1
```
  - Index on (supporter_id, creator_id, state, period_end)
  - Expected: < 50ms p99

---

## Summary: Flow Characteristics

| Characteristic | Value |
|----------------|-------|
| **Duration (publish)** | 2-10 seconds (ACL calculation is slowest step) |
| **Duration (access check)** | 50-200ms (cache hit vs cache miss) |
| **Cross-contract depth** | 5 contracts (content, subscription, feed, notification, reporting) |
| **Critical path (publish)** | Content creation + ACL calculation |
| **Critical path (access)** | Access check (cache or real-time query) |
| **Async components** | Feed addition, notification, reporting (eventual consistency) |
| **Human intervention frequency** | ~5% of cases (moderation, visibility corrections) |
| **Replay risk** | Low (idempotency via content_hash and DB constraints) |
| **Data consistency requirement** | Strong for access enforcement, eventual for feed/notifications |
| **Volume** | 50,000 publishes/day, 10M access checks/day at scale |
| **SLA (access check)** | p99 < 200ms (supporter experience critical) |
| **Security posture** | Fail closed (deny access on errors, never grant by default) |

---