import type { Meta, StoryObj } from "@storybook/react"
import CursorPagination from "./pagination"

const meta: Meta<typeof CursorPagination> = {
    title: "Components/CursorPagination",
    component: CursorPagination,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof CursorPagination>

export const BothCursors: Story = {
    render: () => (
        <CursorPagination
            prevCursor="cursor_prev_abc"
            nextCursor="cursor_next_xyz"
            onPrev={(cursor) => console.log("Prev:", cursor)}
            onNext={(cursor) => console.log("Next:", cursor)}
        />
    ),
}

export const NextOnly: Story = {
    render: () => (
        <CursorPagination
            nextCursor="cursor_next_xyz"
            onPrev={(cursor) => console.log("Prev:", cursor)}
            onNext={(cursor) => console.log("Next:", cursor)}
        />
    ),
}

export const PrevOnly: Story = {
    render: () => (
        <CursorPagination
            prevCursor="cursor_prev_abc"
            onPrev={(cursor) => console.log("Prev:", cursor)}
            onNext={(cursor) => console.log("Next:", cursor)}
        />
    ),
}
