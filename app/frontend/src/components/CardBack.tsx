const leaves = [
  { x: -16, y: 50, r: -18 },
  { x: -28, y: 44, r: -34 },
  { x: -38, y: 35, r: -50 },
  { x: -45, y: 24, r: -66 },
];

/** The shared back of every card: warm card stock with a pen-drawn cross over olive sprigs. */
export function CardBack() {
  return (
    <div className="card-back">
      <svg viewBox="0 0 200 300" aria-hidden="true">
        <rect x="10" y="10" width="180" height="280" rx="10" className="ink-thin" />
        <rect x="17" y="17" width="166" height="266" rx="6" className="ink-hair" />
        <g transform="translate(100 140)">
          <path d="M0 -44 V44 M-21 -16 H21" className="ink" />
          {[1, -1].map((side) => (
            <g key={side} transform={`scale(${side} 1)`}>
              <path d="M-4 56 C-22 50 -40 38 -50 16" className="ink-thin" />
              {leaves.map((l) => (
                <ellipse key={l.x} cx={l.x} cy={l.y} rx="6.5" ry="2.8" transform={`rotate(${l.r} ${l.x} ${l.y})`} className="leaf" />
              ))}
            </g>
          ))}
        </g>
        {[
          [32, 32],
          [168, 32],
          [32, 268],
          [168, 268],
        ].map(([x, y]) => (
          <circle key={`${x}-${y}`} cx={x} cy={y} r="2.2" className="dot" />
        ))}
      </svg>
    </div>
  );
}
