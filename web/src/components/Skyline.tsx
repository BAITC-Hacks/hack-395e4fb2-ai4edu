import {useId} from 'react';
import './skyline.css';

/** Original architectural illustration, not a depiction of real building locations. */
export function Skyline() {
  const id = useId().replace(/:/g, '');
  const fill = (name:string) => `url(#${id}-${name})`;

  return <svg className="astana-skyline" viewBox="0 0 880 340" preserveAspectRatio="xMaxYMax meet" fill="none" aria-hidden="true" focusable="false">
    <defs>
      <linearGradient id={`${id}-sky`} x1="460" y1="0" x2="710" y2="335" gradientUnits="userSpaceOnUse">
        <stop stopColor="#d7e2d1" stopOpacity="0"/><stop offset="1" stopColor="#bacfbd" stopOpacity=".7"/>
      </linearGradient>
      <linearGradient id={`${id}-glass`} x1="620" y1="80" x2="688" y2="282" gradientUnits="userSpaceOnUse">
        <stop stopColor="#a5c2b4"/><stop offset=".5" stopColor="#587b6b"/><stop offset="1" stopColor="#365b4b"/>
      </linearGradient>
      <linearGradient id={`${id}-gold`} x1="533" y1="106" x2="585" y2="154" gradientUnits="userSpaceOnUse">
        <stop stopColor="#f8edbf"/><stop offset=".35" stopColor="#d8bc70"/><stop offset="1" stopColor="#96783a"/>
      </linearGradient>
      <linearGradient id={`${id}-canopy`} x1="740" y1="198" x2="799" y2="276" gradientUnits="userSpaceOnUse">
        <stop stopColor="#f9f6e9"/><stop offset="1" stopColor="#b6c6b0"/>
      </linearGradient>
      <linearGradient id={`${id}-fade`} x1="0" y1="0" x2="880" y2="0" gradientUnits="userSpaceOnUse">
        <stop offset=".1" stopColor="white" stopOpacity="0"/><stop offset=".47" stopColor="white" stopOpacity=".6"/><stop offset=".7" stopColor="white"/>
      </linearGradient>
      <mask id={`${id}-edge`}><rect width="880" height="340" fill={fill('fade')}/></mask>
      <pattern id={`${id}-windows`} width="15" height="18" patternUnits="userSpaceOnUse">
        <path d="M4 4H10V10H4Z" fill="#f5e5ac" opacity=".46"/>
      </pattern>
    </defs>

    <g mask={fill('edge')}>
      <path d="M251 340C231 206 367 99 533 69C672 43 807 82 880 169V340Z" fill={fill('sky')}/>
      <circle cx="751" cy="87" r="53" fill="#f1e8cb" opacity=".6"/>
      {/* Distant silhouettes create depth without competing with the main city scene. */}
      <g fill="#91ab98" opacity=".36">
        <path d="M277 290V208L306 195L334 208V290ZM346 287V181L384 168L410 181V287ZM424 289V221H458V289ZM474 287V190H511V287Z"/>
        <path d="M572 294V206L603 192L629 207V294ZM693 290V184L720 169L745 182V290ZM790 291V213H819V291ZM835 286V159L862 150L883 163V286Z"/>
      </g>
      <path d="M212 299C426 270 677 258 887 277V340H212Z" fill="#c3d0bb"/>
      <path d="M208 315C415 282 666 278 887 294" stroke="#e9eee0" strokeWidth="13"/>
      <path d="M211 318C416 286 663 282 888 297" stroke="#a3b79f" strokeWidth="1.5"/>
      <g fill="#315341" opacity=".13">
        <path d="M343 285L423 279L459 294L379 305Z"/>
        <path d="M505 285L568 282L628 301L560 311Z"/>
        <path d="M613 279L686 273L742 295L665 306Z"/>
        <path d="M707 281L800 273L844 290L755 305Z"/>
      </g>

      {/* Each building has separate roof, lit front face, and darker side face. */}
      <g>
        <path d="M340 284V221L393 206V270Z" fill="#688975"/>
        <path d="M393 206L425 218V282L393 270Z" fill="#3c6050"/>
        <path d="M340 221L370 233L425 218L393 206Z" fill="#afc2a8"/>
        <path d="M351 244L383 235M351 257L383 248M351 270L383 261" stroke="#d6dfc5" strokeWidth="3"/>
        <path d="M402 235L415 240M402 248L415 253M402 261L415 266" stroke="#9bb6a4" strokeWidth="2"/>
      </g>
      <g>
        <path d="M435 284V162L475 150V274Z" fill="#6f907b"/>
        <path d="M475 150L499 161V285L475 274Z" fill="#345747"/>
        <path d="M435 162L458 173L499 161L475 150Z" fill="#d0dac2"/>
        <path d="M443 185V274M454 182V271M465 179V268" stroke="#c2d3ba" strokeWidth="2" opacity=".6"/>
        <path d="M481 178V270M490 182V274" stroke="#9ab29c" strokeWidth="1.5" opacity=".6"/>
        <path d="M450 158V150L470 144L483 150V155L464 161Z" fill="#799883"/>
      </g>

      {/* Abstract gold sphere and branching tower evoke Astana's architecture. */}
      <g>
        <path d="M522 282L548 153H562L586 281L574 286L555 177L535 286Z" fill="#e4e8d6"/>
        <path d="M531 281L550 159M541 282L555 160M569 282L557 160M579 281L561 159" stroke="#819982" strokeWidth="2"/>
        <path d="M539 224H570M534 248H575M530 269H580" stroke="#b0bc9b" strokeWidth="3"/>
        <ellipse cx="555" cy="282" rx="35" ry="8" fill="#9daf94"/>
        <ellipse cx="555" cy="278" rx="31" ry="7" fill="#e4e8d4"/>
        <circle cx="555" cy="130" r="30" fill={fill('gold')}/>
        <path d="M526 132C541 144 571 144 584 131M536 107C535 127 542 148 555 159M550 101C544 124 553 145 569 154" stroke="#f6e5a7" strokeWidth="1" opacity=".54"/>
        <ellipse cx="544" cy="117" rx="11" ry="5" transform="rotate(-36 544 117)" fill="#fff8d5" opacity=".45"/>
      </g>

      {/* Glass towers keep the same muted palette as the interactive city. */}
      <g>
        <path d="M614 275V101L650 83L671 99V282Z" fill={fill('glass')}/>
        <path d="M650 83L682 99V276L671 282V99Z" fill="#315748"/>
        <path d="M614 101L649 112L682 99L650 83Z" fill="#cbded0"/>
        <path d="M623 109V275M634 112V278M645 116V279M658 112V281" stroke="#cadbc9" strokeWidth="1.5" opacity=".7"/>
        <path d="M619 139L667 153M619 170L667 184M619 201L667 215M619 232L667 246" stroke="#c4d4bc" opacity=".4"/>
        <path d="M623 259L656 269" stroke="#e2c97c" strokeWidth="2"/>
        <path d="M681 275V174L711 162L728 172V279L711 286Z" fill="#567762"/>
        <path d="M711 162L728 172V279L711 286Z" fill="#2e5041"/>
        <path d="M681 174L698 182L728 172L711 162Z" fill="#afc4ab"/>
        <path d="M689 186V273M699 190V278" stroke="#bccdb4" strokeWidth="3" opacity=".5"/>
      </g>

      {/* A sculptural canopy; its position is purely part of the illustration. */}
      <g>
        <path d="M721 278Q750 247 765 185Q776 236 814 272L778 287Z" fill={fill('canopy')}/>
        <path d="M765 185Q766 239 778 287L814 272Q780 234 765 185Z" fill="#9caf99"/>
        <path d="M728 273Q750 245 765 185Q764 245 756 281M765 185Q774 239 796 277" stroke="#eaf0dc" strokeWidth="2"/>
        <path d="M765 185V164" stroke="#829879" strokeWidth="2"/>
        <path d="M721 278L778 287L814 272V279L778 294L721 284Z" fill="#6d8b71"/>
        <path d="M729 281L775 289M783 286L808 277" stroke="#dcc886" strokeWidth="2"/>
      </g>
      <g>
        <path d="M812 280V221L847 210L873 221V281L847 291Z" fill="#54735c"/>
        <path d="M847 210L873 221V281L847 291Z" fill="#2d5040"/>
        <path d="M812 221L838 232L873 221L847 210Z" fill="#b6c8a7"/>
        <path d="M818 239H842V278H818Z" fill={fill('windows')}/>
        <path d="M854 237V274M863 241V277" stroke="#a6baa0" strokeWidth="2"/>
      </g>

      {/* Foreground gardens and a quiet embankment complete the depth layers. */}
      <g fill="#638360">
        <ellipse cx="408" cy="302" rx="20" ry="6" fill="#a7bb99"/>
        <path d="M404 298V286M415 298V280" stroke="#466745" strokeWidth="3"/>
        <circle cx="404" cy="281" r="9"/><circle cx="415" cy="275" r="12" fill="#84a17a"/>
        <ellipse cx="708" cy="309" rx="21" ry="6" fill="#a7bb99"/>
        <path d="M703 307V294M716 307V290" stroke="#466745" strokeWidth="3"/>
        <circle cx="703" cy="287" r="11"/><circle cx="716" cy="283" r="13" fill="#84a17a"/>
        <ellipse cx="848" cy="316" rx="19" ry="6" fill="#a7bb99"/>
        <path d="M846 313V298" stroke="#466745" strokeWidth="3"/><circle cx="846" cy="291" r="13" fill="#78966f"/>
      </g>
      <path d="M381 340C517 312 743 313 888 328V340H381Z" fill="#8daea0" opacity=".65"/>
      <path d="M417 335C553 314 743 319 885 332" stroke="#e1e9d9" strokeWidth="2"/>
      <path d="M552 334L600 331M757 335L816 338" stroke="#c7d7c7" strokeWidth="2"/>
    </g>
  </svg>;
}
