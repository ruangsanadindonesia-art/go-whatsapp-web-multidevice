/** Gold hairline – rotated-square diamond – gold hairline (Ruang Sanad motif). */
export function Ornament({ className = '' }: { className?: string }) {
  return (
    <div aria-hidden className={'flex items-center gap-2 ' + className}>
      <span className="h-px w-10 bg-gold-500" />
      <span className="size-1.5 rotate-45 bg-gold-500" />
      <span className="h-px w-10 bg-gold-500" />
    </div>
  )
}

/** Page heading: eyebrow, Playfair title with an italic gold accent phrase, ornament. */
export function PageHeading({ eyebrow, title, accent }: { eyebrow: string; title: string; accent?: string }) {
  return (
    <header className="mb-6 md:mb-8">
      <p className="eyebrow">{eyebrow}</p>
      <h1 className="mt-2 line-clamp-3 font-display text-[24px] leading-tight font-semibold tracking-[-0.01em] text-teal-900 [text-wrap:balance] md:text-[28px]">
        {title} {accent && <em className="text-gold-600 italic">{accent}</em>}
      </h1>
      <Ornament className="mt-3" />
    </header>
  )
}
