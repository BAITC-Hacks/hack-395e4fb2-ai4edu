// Original line drawing: Astana skyline, deliberately not a district map.
export function Skyline() {
  return <svg className="skyline" viewBox="0 0 640 180" fill="none" aria-hidden="true">
    <circle cx="464" cy="65" r="47" fill="currentColor" opacity=".07"/>
    <g stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round">
      <path d="M0 147H640M0 165Q170 138 320 166T640 163" opacity=".35"/>
      <path d="M25 147V94H57V147M34 105H48M34 117H48M68 147V78H104V147M77 90H96M77 103H96M77 116H96M117 147V109H157V147"/>
      <path d="M164 147L227 69L280 147ZM184 147L227 69L259 147M227 69V51"/>
      <path d="M304 147L316 66M337 147L325 66M311 108H331M307 129H335M318 64V142M323 64V142"/>
      <circle cx="321" cy="46" r="20" fill="currentColor" fillOpacity=".16"/>
      <path d="M297 147H345M362 147V99L380 78L397 99V147M368 106H390M368 117H390M411 147V82H442V147M418 93H435M418 105H435"/>
      <path d="M466 147L496 34L529 147ZM482 147L496 34L513 147M496 34V18M475 112H519M470 130H525"/>
      <path d="M549 147V108H579V147M585 147V89H616V147M594 101H608M594 114H608"/>
    </g>
  </svg>;
}
