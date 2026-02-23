import { useCallback, useEffect, useRef, useState } from "react"

interface UseCountUpOptions {
  to: number
  from?: number
  duration?: number
  startWhen?: boolean
  separator?: string
  decimals?: number
}

const prefersReducedMotion = () =>
  typeof window !== "undefined" &&
  window.matchMedia("(prefers-reduced-motion: reduce)").matches

function formatNumber(value: number, separator: string, decimals: number): string {
  if (separator === ",") {
    return new Intl.NumberFormat("en-US", {
      minimumFractionDigits: decimals,
      maximumFractionDigits: decimals,
    }).format(value)
  }
  const formatted = new Intl.NumberFormat("en-US", {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(value)
  return formatted.replace(/,/g, separator)
}

export function useCountUp({
  to,
  from = 0,
  duration = 0.8,
  startWhen = true,
  separator = ",",
  decimals = 0,
}: UseCountUpOptions) {
  const [value, setValue] = useState(from)
  const rafRef = useRef<number>(0)
  const startTimeRef = useRef<number>(0)
  const currentFromRef = useRef(from)
  const hasAnimatedRef = useRef(false)
  const prevToRef = useRef(to)

  const animate = useCallback(
    (animFrom: number, animTo: number) => {
      if (prefersReducedMotion()) {
        setValue(animTo)
        return
      }

      const durationMs = duration * 1000
      startTimeRef.current = 0

      const step = (timestamp: number) => {
        if (!startTimeRef.current) startTimeRef.current = timestamp
        const elapsed = timestamp - startTimeRef.current
        const t = Math.min(elapsed / durationMs, 1)
        const eased = 1 - Math.pow(1 - t, 3)
        const current = animFrom + (animTo - animFrom) * eased

        setValue(current)

        if (t < 1) {
          rafRef.current = requestAnimationFrame(step)
        } else {
          setValue(animTo)
        }
      }

      cancelAnimationFrame(rafRef.current)
      rafRef.current = requestAnimationFrame(step)
    },
    [duration],
  )

  useEffect(() => {
    if (!startWhen) {
      setValue(from)
      return
    }

    if (!hasAnimatedRef.current) {
      hasAnimatedRef.current = true
      currentFromRef.current = from
      animate(from, to)
    }

    return () => cancelAnimationFrame(rafRef.current)
  }, [startWhen, from, to, animate])

  useEffect(() => {
    if (!hasAnimatedRef.current || !startWhen) return
    if (prevToRef.current === to) return

    currentFromRef.current = value
    prevToRef.current = to
    animate(value, to)

    return () => cancelAnimationFrame(rafRef.current)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [to])

  return {
    value,
    formatted: formatNumber(value, separator, decimals),
  }
}
