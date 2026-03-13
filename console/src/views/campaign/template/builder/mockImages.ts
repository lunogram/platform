import type { Image } from "@/types"
import type { UUID } from "@/types/common"

/**
 * Deterministic mock project ID — shared across all mock images.
 * Using a static UUID avoids the instability of `createUuid()` at module scope,
 * which generates new IDs on every import / HMR reload and breaks selectedIds
 * matching in ImageLibraryModal.
 */
const MOCK_PROJECT_ID = "00000000-0000-4000-8000-000000000000" as UUID

/** Pre-generated mock images using picsum.photos for the library */
export const MOCK_IMAGES: (Image & { description?: string })[] = [
    {
        id: "a0000001-0001-4000-8000-000000000001" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/brand-logo/400/200",
        name: "Company Logo",
        filename: "company-logo.png",
        key: "images/company-logo.png",
        content_type: "image/png",
        size_bytes: 24_500,
        created_at: "2025-12-01T10:00:00Z",
        updated_at: "2025-12-01T10:00:00Z",
        description: "Minimalist company logo with bold sans-serif text on a white background",
    },
    {
        id: "a0000001-0001-4000-8000-000000000002" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/hero-product/600/400",
        name: "Product Dashboard",
        filename: "product-dashboard.png",
        key: "images/product-dashboard.png",
        content_type: "image/png",
        size_bytes: 185_200,
        created_at: "2025-12-02T14:30:00Z",
        updated_at: "2025-12-02T14:30:00Z",
        description:
            "Dark analytics dashboard with charts, sidebar navigation, and key metrics cards",
    },
    {
        id: "a0000001-0001-4000-8000-000000000003" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/team-photo/600/400",
        name: "Team Photo",
        filename: "team-photo.jpg",
        key: "images/team-photo.jpg",
        content_type: "image/jpeg",
        size_bytes: 312_400,
        created_at: "2025-12-03T09:15:00Z",
        updated_at: "2025-12-03T09:15:00Z",
        description: "Diverse team of five people collaborating around a laptop in a modern office",
    },
    {
        id: "a0000001-0001-4000-8000-000000000004" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/promo-banner/800/300",
        name: "Promo Banner",
        filename: "promo-banner.png",
        key: "images/promo-banner.png",
        content_type: "image/png",
        size_bytes: 97_800,
        created_at: "2025-12-04T16:00:00Z",
        updated_at: "2025-12-04T16:00:00Z",
        description:
            "Vibrant gradient banner with large sale text and geometric shapes in purple and orange",
    },
    {
        id: "a0000001-0001-4000-8000-000000000005" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/app-screenshot/400/600",
        name: "Mobile App Screenshot",
        filename: "app-screenshot.png",
        key: "images/app-screenshot.png",
        content_type: "image/png",
        size_bytes: 145_600,
        created_at: "2025-12-05T11:20:00Z",
        updated_at: "2025-12-05T11:20:00Z",
        description:
            "iPhone mockup showing a clean onboarding screen with illustration and sign-up form",
    },
    {
        id: "a0000001-0001-4000-8000-000000000006" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/office-space/600/400",
        name: "Office Interior",
        filename: "office-interior.jpg",
        key: "images/office-interior.jpg",
        content_type: "image/jpeg",
        size_bytes: 276_300,
        created_at: "2025-12-06T08:45:00Z",
        updated_at: "2025-12-06T08:45:00Z",
        description:
            "Bright open-plan office with standing desks, plants, and natural light from large windows",
    },
    {
        id: "a0000001-0001-4000-8000-000000000007" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/celebration/600/400",
        name: "Celebration",
        filename: "celebration.jpg",
        key: "images/celebration.jpg",
        content_type: "image/jpeg",
        size_bytes: 198_700,
        created_at: "2025-12-07T13:00:00Z",
        updated_at: "2025-12-07T13:00:00Z",
        description: "Confetti and streamers against a dark background, festive celebration mood",
    },
    {
        id: "a0000001-0001-4000-8000-000000000008" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/abstract-bg/800/600",
        name: "Abstract Background",
        filename: "abstract-bg.png",
        key: "images/abstract-bg.png",
        content_type: "image/png",
        size_bytes: 67_400,
        created_at: "2025-12-08T17:30:00Z",
        updated_at: "2025-12-08T17:30:00Z",
        description: "Soft gradient abstract background with flowing blue and teal organic shapes",
    },
    {
        id: "a0000001-0001-4000-8000-000000000009" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/headshot/400/400",
        name: "CEO Headshot",
        filename: "ceo-headshot.jpg",
        key: "images/ceo-headshot.jpg",
        content_type: "image/jpeg",
        size_bytes: 42_100,
        created_at: "2025-12-09T10:00:00Z",
        updated_at: "2025-12-09T10:00:00Z",
        description:
            "Professional headshot of a person in business attire with a neutral studio background",
    },
    {
        id: "a0000001-0001-4000-8000-00000000000a" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/social-proof/600/400",
        name: "Customer Testimonial",
        filename: "testimonial-card.png",
        key: "images/testimonial-card.png",
        content_type: "image/png",
        size_bytes: 88_900,
        created_at: "2025-12-10T14:00:00Z",
        updated_at: "2025-12-10T14:00:00Z",
        description:
            "Screenshot of a five-star review card with quote text and a small customer avatar",
    },
    {
        id: "a0000001-0001-4000-8000-00000000000b" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/feature-icon/200/200",
        name: "Feature Icon Set",
        filename: "feature-icons.svg",
        key: "images/feature-icons.svg",
        content_type: "image/svg+xml",
        size_bytes: 12_300,
        created_at: "2025-12-11T09:00:00Z",
        updated_at: "2025-12-11T09:00:00Z",
        description: "Set of line-style icons for speed, security, and analytics features",
    },
    {
        id: "a0000001-0001-4000-8000-00000000000c" as UUID,
        project_id: MOCK_PROJECT_ID,
        url: "https://picsum.photos/seed/landscape/800/400",
        name: "Landscape Banner",
        filename: "landscape-hero.jpg",
        key: "images/landscape-hero.jpg",
        content_type: "image/jpeg",
        size_bytes: 342_000,
        created_at: "2025-12-12T15:45:00Z",
        updated_at: "2025-12-12T15:45:00Z",
        description:
            "Dramatic mountain landscape at sunset with warm golden light and misty valleys",
    },
]

/**
 * Simulate fetching images from the library.
 * In production this calls api.images.search().
 */
export async function fetchMockImages(
    query?: string,
): Promise<(Image & { description?: string })[]> {
    // Simulate network delay
    await new Promise((resolve) => setTimeout(resolve, 300))

    if (!query || query.trim() === "") {
        return MOCK_IMAGES
    }

    const q = query.toLowerCase()
    return MOCK_IMAGES.filter(
        (img) =>
            img.name.toLowerCase().includes(q) ||
            img.filename.toLowerCase().includes(q) ||
            (img.description && img.description.toLowerCase().includes(q)),
    )
}
