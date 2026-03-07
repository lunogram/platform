import {
    Pagination,
    PaginationContent,
    PaginationItem,
    PaginationPrevious,
    PaginationNext,
} from "@/components/ui/pagination"

interface PaginationProps {
    prevCursor?: string
    nextCursor?: string
    onPrev: (cursor?: string) => void
    onNext: (cursor?: string) => void
}

export default function CursorPagination({
    prevCursor,
    nextCursor,
    onPrev,
    onNext,
}: PaginationProps) {
    if (!prevCursor && !nextCursor) return <></>
    return (
        <Pagination className="mt-4">
            <PaginationContent>
                <PaginationItem>
                    <PaginationPrevious
                        onClick={(e) => {
                            e.preventDefault()
                            onPrev(prevCursor)
                        }}
                        aria-disabled={prevCursor === undefined}
                        className={
                            prevCursor === undefined
                                ? "pointer-events-none opacity-50"
                                : "cursor-pointer"
                        }
                    />
                </PaginationItem>
                <PaginationItem>
                    <PaginationNext
                        onClick={(e) => {
                            e.preventDefault()
                            onNext(nextCursor)
                        }}
                        aria-disabled={nextCursor === undefined}
                        className={
                            nextCursor === undefined
                                ? "pointer-events-none opacity-50"
                                : "cursor-pointer"
                        }
                    />
                </PaginationItem>
            </PaginationContent>
        </Pagination>
    )
}
