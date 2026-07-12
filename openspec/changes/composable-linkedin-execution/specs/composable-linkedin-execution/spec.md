## ADDED Requirements

### Requirement: LinkedIn Content Studio is a linear composable execution
The `linkedin-content-studio` PlanTemplate SHALL declare the linear sequence `fetch-news`,
`write-draft`, `author-content`, optional `draft-carousel`, optional `generate-image`, and
`publish`, where `author-content` and both enrichment steps resolve to `LinkedInPost` and
`publish` accepts exactly one final `LinkedInPost`.

#### Scenario: Enabled enrichments run before approval and publish
- **WHEN** carousel and image capabilities are included for a runnable execution
- **THEN** execution order is fetch-news, write-draft, author-content, draft-carousel, generate-image, approval, publish
- **AND** publish receives the final `LinkedInPost` version after both accepted enrichment reviews

#### Scenario: Images are excluded from the active graph
- **WHEN** image generation is not included
- **THEN** no image executor is invoked
- **AND** `generate-image` is absent from the immutable active execution graph rendered to the user

### Requirement: Approval pins the final composed post
The publish approval request SHALL be created only after every active significant-content review
has been accepted and SHALL pin the final `LinkedInPost` ArtifactRef, including its version id and
content hash.

#### Scenario: Approval previews the exact final carousel post
- **WHEN** a carousel is active and publish reaches its approval gate
- **THEN** the approval subject is the final composable `LinkedInPost` ArtifactRef
- **AND** its carousel preview is available before the user decides

#### Scenario: Later current-version changes cannot alter publish input
- **WHEN** a different ArtifactVersion becomes current after the approval request was created
- **THEN** approving the request still loads and publishes the pinned version and hash
- **AND** the later version is not sent to LinkedIn

### Requirement: LinkedIn publishing uses the real text and document API paths
The LinkedIn integration adapter SHALL publish a pinned text-only post through the real LinkedIn
REST post endpoint, and SHALL publish a pinned carousel by initializing document upload, uploading
the pinned document bytes, and creating one document post referencing the returned document URN.

#### Scenario: Approve publishes exact text-only content once
- **WHEN** the pinned final post contains no carousel and the approval is approved
- **THEN** the publisher is invoked once with text derived only from the pinned post version
- **AND** it creates one real LinkedIn text post request

#### Scenario: Approve uploads and publishes exact carousel bytes once
- **WHEN** the pinned final post contains a carousel
- **THEN** the publisher initializes a LinkedIn document upload, PUTs exactly the pinned carousel-document bytes, and creates one post referencing the returned document URN
- **AND** it does not issue a text-only publish request

#### Scenario: Reject or cancel makes no external call
- **WHEN** the publish approval is rejected or the execution is cancelled before approval
- **THEN** the LinkedIn publisher is invoked zero times
- **AND** no document initialization, upload, or post request is made
