# unified-content-post

## Purpose
The LinkedIn output is a single composable post artifact that may carry a text body, an
optional carousel (markup slides), and optional generated image(s); one publish step consumes
it.

## ADDED Requirements

### Requirement: One post artifact
The LinkedIn content output SHALL be a single `LinkedInPost` artifact composing a text body,
an optional `CarouselDraft`, and optional `ImageAsset`(s).

#### Scenario: text-only post
- GIVEN a run with no carousel and no image opt-in
- THEN the post carries only the text body

#### Scenario: post with carousel
- GIVEN a run where the specialist authored a carousel
- THEN the post carries the text body and the carousel draft

### Requirement: One publish step
There SHALL be a single publish step that publishes whatever the post carries; the per-format
`publish-post`/`publish-carousel` split is removed.

#### Scenario: single publish consumes the post
- GIVEN a post carrying text + carousel
- WHEN publish runs
- THEN one publish step publishes the whole post (not separate publishes per content part)

### Requirement: Carousel is markup
The carousel SHALL be a `CarouselDraft` (structured slides) previewable as markdown; HTML/CSS→PDF
rendering is an additive future layer over the same payload (not required here).

#### Scenario: carousel preview
- GIVEN a post with a carousel
- WHEN previewed
- THEN it renders the slides as markdown sections (headings + bodies)

### Requirement: Image is opt-in content
Generated images SHALL appear in the post only when the user opted into image generation; the
Image Generator agent's `ImageAsset` outputs embed into the post/carousel.

#### Scenario: no image when opted out
- GIVEN a run where image generation was not selected
- THEN the post carries no generated images and the Image Generator agent is not part of the team
