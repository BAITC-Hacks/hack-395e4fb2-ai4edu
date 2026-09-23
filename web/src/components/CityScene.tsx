import {useId,type CSSProperties} from 'react';
import './city-scene.css';

// Original illustration, authored in scene space. These coordinates, heights and
// footprints contain NO scenario metrics, construction locations or measure counts.
// The ground/data adapter lives separately in districtGeometry.ts.
type Slot='saryarka'|'almaty'|'esil'|'nura'|'baikonur';
type Tone='forest'|'sage'|'ivory'|'glass';
interface BuildingSpec {id:string;slot:Slot;x:number;y:number;w:number;d:number;h:number;delay:number;tone:Tone;roof?:'garden'|'terrace'|'solar'|'spire'}
type SceneStyle=CSSProperties&{'--city-delay':string};
const timed=(delay:number):SceneStyle=>({'--city-delay':`${delay}s`});
const plane='M440 130 980 424 620 629 80 335 Z';
const river='M275 230 C348 263 383 287 450 318 S602 391 670 432 S778 482 834 510';

// Ground anchor, width, depth and height are explicit art direction, not random
// generation. Back-to-front ordering is based only on this fixed scene geometry.
const buildings:BuildingSpec[]=([
  {id:'courtyard-north',slot:'saryarka',x:476,y:190,w:32,d:24,h:45,delay:.05,tone:'ivory',roof:'terrace'},
  {id:'studio',slot:'saryarka',x:535,y:226,w:25,d:27,h:70,delay:.28,tone:'forest',roof:'solar'},
  {id:'garden-house',slot:'saryarka',x:400,y:213,w:33,d:25,h:52,delay:.5,tone:'sage',roof:'garden'},
  {id:'north-block',slot:'saryarka',x:467,y:260,w:35,d:31,h:89,delay:.68,tone:'forest',roof:'terrace'},
  {id:'west-studio',slot:'saryarka',x:319,y:224,w:25,d:25,h:32,delay:.85,tone:'ivory',roof:'solar'},
  {id:'riverside-house',slot:'saryarka',x:390,y:259,w:30,d:23,h:38,delay:1.04,tone:'sage',roof:'garden'},
  {id:'east-tower',slot:'almaty',x:659,y:281,w:31,d:27,h:97,delay:.45,tone:'forest',roof:'spire'},
  {id:'east-block',slot:'almaty',x:720,y:311,w:31,d:27,h:66,delay:.65,tone:'ivory',roof:'terrace'},
  {id:'glass-crown',slot:'almaty',x:789,y:348,w:25,d:26,h:104,delay:.9,tone:'glass',roof:'spire'},
  {id:'river-tower',slot:'almaty',x:602,y:318,w:27,d:30,h:153,delay:1.17,tone:'glass',roof:'spire'},
  {id:'stepped-tower',slot:'almaty',x:666,y:366,w:29,d:28,h:128,delay:1.42,tone:'forest',roof:'garden'},
  {id:'east-pavilion',slot:'almaty',x:870,y:396,w:36,d:24,h:40,delay:1.55,tone:'ivory',roof:'solar'},
  {id:'east-gallery',slot:'almaty',x:832,y:448,w:40,d:26,h:36,delay:1.9,tone:'sage',roof:'garden'},
  {id:'west-gallery',slot:'esil',x:279,y:267,w:39,d:26,h:33,delay:1.25,tone:'ivory',roof:'terrace'},
  {id:'civic-hall',slot:'esil',x:250,y:303,w:38,d:45,h:36,delay:1.55,tone:'forest',roof:'garden'},
  {id:'west-apartments',slot:'esil',x:357,y:318,w:29,d:25,h:57,delay:1.8,tone:'sage',roof:'solar'},
  {id:'terraced-homes',slot:'esil',x:182,y:342,w:31,d:25,h:26,delay:2.05,tone:'ivory',roof:'garden'},
  {id:'corner-house',slot:'esil',x:264,y:391,w:30,d:23,h:30,delay:2.3,tone:'forest',roof:'terrace'},
  {id:'central-house',slot:'nura',x:454,y:378,w:29,d:30,h:66,delay:1.75,tone:'ivory',roof:'garden'},
  {id:'central-tower',slot:'nura',x:535,y:422,w:29,d:27,h:86,delay:2.05,tone:'glass',roof:'solar'},
  {id:'central-corner',slot:'nura',x:569,y:453,w:30,d:24,h:46,delay:2.4,tone:'sage',roof:'terrace'},
  {id:'park-house',slot:'nura',x:326,y:441,w:29,d:27,h:36,delay:2.6,tone:'forest',roof:'garden'},
  {id:'south-tower',slot:'baikonur',x:641,y:473,w:29,d:27,h:68,delay:2.25,tone:'forest',roof:'solar'},
  {id:'south-gallery',slot:'baikonur',x:726,y:494,w:39,d:24,h:37,delay:2.5,tone:'ivory',roof:'garden'},
  {id:'south-block',slot:'baikonur',x:764,y:536,w:28,d:25,h:52,delay:2.8,tone:'sage',roof:'terrace'},
  {id:'waterfront-house',slot:'baikonur',x:535,y:542,w:32,d:26,h:43,delay:2.95,tone:'glass',roof:'solar'},
  {id:'front-pavilion',slot:'baikonur',x:627,y:585,w:39,d:26,h:37,delay:3.1,tone:'forest',roof:'garden'},
] satisfies BuildingSpec[]).sort((a,b)=>a.y-b.y);

export function CityGround(){
  const id=useId();
  return <g className="city-ground" aria-hidden="true" pointerEvents="none">
    <defs>
      <pattern id={`${id}-grid`} width="88" height="48" patternUnits="userSpaceOnUse"><path d="M0 24 44 0 88 24 44 48 Z" fill="none" stroke="#75927e" strokeWidth=".7"/></pattern>
      <radialGradient id={`${id}-shadow`}><stop offset="0" stopColor="#244d3c" stopOpacity=".15"/><stop offset="1" stopColor="#244d3c" stopOpacity="0"/></radialGradient>
    </defs>
    <ellipse cx="532" cy="493" rx="493" ry="192" fill={`url(#${id}-shadow)`}/>
    <path d="M40 310 432 96 1020 416 628 667 Z" fill={`url(#${id}-grid)`} opacity=".2"/>
    <path d="M80 335 620 629 980 424 980 436 620 642 80 347 Z" fill="#a6b7a6"/>
    <path d="M80 335 620 629 620 642 80 347 Z" fill="#c3ccba"/>
    <path d={plane} fill="#e4e8d7" stroke="#d1dcc9" strokeWidth="2"/>
    <path d="M82 346 620 639 978 435" fill="none" stroke="#899f8f" strokeWidth="1" opacity=".5"/>
  </g>;
}

function Building({b,affected}:{b:BuildingSpec;affected:boolean}){
  const {w,d,h}=b,leftY=-w*.48,rightY=-d*.55;
  const footprint=`${-w},${leftY} ${d-w},${leftY+rightY} ${d},${rightY} 0,0`;
  const roof=`${-w},${leftY-h} ${d-w},${leftY+rightY-h} ${d},${rightY-h} 0,${-h}`;
  const rows=Math.max(2,Math.floor(h/15)),columns=b.tone==='glass'?4:3;
  return <g transform={`translate(${b.x} ${b.y})`} className={`city-building city-tone-${b.tone}${affected?' is-affected':''}`} data-city-slot={b.slot} style={timed(b.delay)}>
    <polygon points={`${-w+8},${leftY+3} ${d+19},${rightY+11} 21,18 ${-w+8},${leftY+18}`} fill="#163f31" opacity=".13"/>
    <g className="city-foundation"><polygon points={footprint} fill="#d2c4a0" stroke="#938c6e" strokeWidth="1"/><path d={`M${-w+4} ${leftY+2} 0 2 ${d+3} ${rightY+2}`} fill="none" stroke="#f5e6b8" strokeWidth="2"/></g>
    <g className="city-frame" fill="none" stroke="#9d9c76" strokeWidth="1.15">
      <path d={`M${-w} ${leftY} V${leftY-h} L0 ${-h} ${d} ${rightY-h} V${rightY} M0 0 V${-h} M${-w} ${leftY-h} ${d-w} ${leftY+rightY-h} ${d} ${rightY-h}`}/>
      {[.25,.5,.75].map(level=><path key={level} d={`M${-w} ${leftY-h*level} 0 ${-h*level} ${d} ${rightY-h*level}`}/>)}
    </g>
    <g className="city-shell">
      <path className="city-face-left" d={`M${-w} ${leftY} 0 0 V${-h} L${-w} ${leftY-h} Z`}/>
      <path className="city-face-right" d={`M0 0 ${d} ${rightY} V${rightY-h} L0 ${-h} Z`}/>
      <polygon points={roof} className="city-roof"/>
      <path d={`M${-w} ${leftY-h} 0 ${-h} ${d} ${rightY-h}`} className="city-roof-rim" fill="none" strokeWidth="1.35"/>
      {b.tone==='glass'&&<><path d={`M${-w*.72} ${leftY*.72-4} V${leftY*.72-h+3} M${d*.46} ${rightY*.46-4} V${rightY*.46-h+3}`} stroke="#cce4ca" strokeWidth="2" opacity=".35"/><path d={`M${-w+2} ${leftY-h+4} 0 ${-h+4} ${d-2} ${rightY-h+4}`} fill="none" stroke="#d7e8cd" strokeWidth="1.5"/></>}
      {b.roof==='solar'&&<g transform={`translate(0 ${-h-2})`}><path d={`M${-w*.6} ${leftY*.55} ${-w*.23} ${leftY*.35-5} ${d*.46-w*.23} ${leftY*.35+rightY*.46-5} ${d*.46-w*.6} ${leftY*.55+rightY*.46} Z`} fill="#2d5146" stroke="#cad7b7" strokeWidth=".7"/><path d={`M${-w*.43} ${leftY*.45-2} ${d*.46-w*.43} ${leftY*.45+rightY*.46-2}`} stroke="#92b2a0" strokeWidth=".6"/></g>}
      {b.roof==='garden'&&<g transform={`translate(0 ${-h-1})`}><path d={`M${-w*.72} ${leftY*.75} ${d*.65-w*.72} ${leftY*.75+rightY*.65} ${d*.65-w*.15} ${leftY*.15+rightY*.65} ${-w*.15} ${leftY*.15} Z`} fill="#9baf7d"/><ellipse cx={-w*.4+d*.25} cy={leftY*.4+rightY*.25-1} rx="5" ry="3" fill="#527f51"/><ellipse cx={-w*.55+d*.5} cy={leftY*.55+rightY*.5-1} rx="4" ry="2.5" fill="#6d9464"/></g>}
      {b.roof==='terrace'&&<path d={`M${-w*.5} ${leftY*.5-h-1} V${leftY*.5-h-5} L${d*.42-w*.5} ${leftY*.5+rightY*.42-h-5} ${d*.42-w*.15} ${leftY*.15+rightY*.42-h-5} V${leftY*.15+rightY*.42-h-1}`} fill="#cdd5bc" stroke="#879c84" strokeWidth=".7"/>}
      {b.roof==='spire'&&<g><path d={`M${(d-w)*.4} ${(leftY+rightY)*.4-h} v-17`} stroke="#dbc68f" strokeWidth="1.6"/><circle cx={(d-w)*.4} cy={(leftY+rightY)*.4-h-18} r="2" fill="#e1cc8a"/></g>}
    </g>
    <g className="city-windows">
      {Array.from({length:rows},(_,row)=>Array.from({length:columns},(_,col)=>{
        const u=(col+.55)/columns,fw=w/(columns*2.3),fh=b.tone==='glass'?7:3.5;
        const x=-w+w*u,y=leftY*(1-u)-h*(row+.65)/rows;
        const rx=d*u,ry=rightY*u-h*(row+.65)/rows,rw=d/(columns*2.5);
        return <g key={`${row}-${col}`}><path d={`M${x} ${y} l${fw} ${fw*.48} v${fh} l${-fw} ${-fw*.48} Z`} fill={(row+col)%4===0?'#e8d79e':'#bbceba'} opacity={b.tone==='glass'?.5:.64}/><path d={`M${rx} ${ry} l${rw} ${-rw*.55} v${fh} l${-rw} ${rw*.55} Z`} fill={(row+col)%3===0?'#efd995':'#98b6aa'} opacity=".67"/></g>;
      }))}
    </g>
    {affected&&<path className="city-renovation-accent" d={`M${-w} ${leftY-h-2} 0 ${-h-2} ${d} ${rightY-h-2}`} fill="none" stroke="#dab861" strokeWidth="2.2"/>}
  </g>;
}

function Tree({x,y,size=1}:{x:number;y:number;size?:number}){
  return <g transform={`translate(${x} ${y}) scale(${size})`}><ellipse rx="10" ry="4.5" cx="4" cy="1" fill="#264b3620"/><path d="M0 1 V-14" stroke="#647655" strokeWidth="2"/><path d="M0-30C-15-20-14-9 0-6 14-9 15-20 0-30Z" fill="#628259"/><path d="M0-30C-10-19-7-10 0-6Z" fill="#9eb382"/></g>;
}
function Park({x,y,delay=2.5}:{x:number;y:number;delay?:number}){
  return <g transform={`translate(${x} ${y})`} className="city-landscape" style={timed(delay)}><path d="M-42-12 5-39 58-10 11 17Z" fill="#b4c59a"/><path d="M-26-10 7 8 42-12 M-7-29 23-13 4-2" fill="none" stroke="#e5dec4" strokeWidth="5"/><Tree x={-19} y={-9} size={.75}/><Tree x={10} y={-23} size={.85}/><Tree x={35} y={-4} size={.7}/><path d="M-5 9 8 16 M18 1 30 8" stroke="#7d8c67" strokeWidth="2.5"/></g>;
}

export function CityArchitecture({affectedSlots=[]}:{affectedSlots?:readonly string[]}){
  return <g className="city-architecture" aria-hidden="true" pointerEvents="none">
    <g className="city-streets" fill="none" strokeLinejoin="round">
      <path d="M438 140 968 429 M98 334 620 618 M638 245 288 443 M804 336 459 533 M216 264 748 552" stroke="#edf0df" strokeWidth="15"/>
      <path d="M438 140 968 429 M98 334 620 618 M638 245 288 443 M804 336 459 533 M216 264 748 552" stroke="#adb7a0" strokeWidth=".8"/>
      <path d={river} stroke="#d5e5ce" strokeWidth="49"/>
      <path d={river} stroke="#81a299" strokeWidth="34"/>
      <path d={river} stroke="#abc6b5" strokeWidth="1.2"/>
      <path d="M309 240 365 269 M482 339 543 371 M699 450 758 482" stroke="#c5dcc6" strokeWidth="2"/>
      <g className="city-bridges"><path d="M443 280 372 320 M606 369 534 410 M782 467 714 506" stroke="#657f71" strokeWidth="19"/><path d="M443 280 372 320 M606 369 534 410 M782 467 714 506" stroke="#e8dfbe" strokeWidth="14"/><path d="M443 280 372 320 M606 369 534 410 M782 467 714 506" stroke="#9a9f84" strokeWidth="1" strokeDasharray="4 5"/><path d="M439 275 368 315 M447 285 376 325 M602 364 530 405 M610 374 538 415 M778 462 710 501 M786 472 718 511" stroke="#eff0d6" strokeWidth="2.5"/></g>
      <path className="city-transit" d="M221 270 745 555" stroke="#d0b76d" strokeWidth="3"/>
      <path className="city-transit" d="M221 270 745 555" stroke="#f7edb9" strokeWidth=".9"/>
      <g className="city-transit" fill="#f7f1d5" stroke="#aa9d6c" strokeWidth="1.2"><path d="M401 363 418 354 442 367 425 377Z"/><path d="M631 488 648 479 672 492 655 502Z"/></g>
    </g>
    <Park x={548} y={258} delay={.4}/>
    <Park x={191} y={292} delay={1.6}/>
    <Park x={915} y={431} delay={1.9}/>
    <Park x={363} y={483} delay={2.8}/>
    <Park x={555} y={588} delay={3.2}/>
    <g className="city-plaza" fill="none" stroke="#e4dec4" strokeWidth="2"><path d="M683 314 720 334 683 355 646 335Z"/><path d="M689 323 708 334 684 348 661 335Z"/><path d="M481 527 513 545 481 563 449 545Z"/></g>
    {buildings.map(b=><Building key={b.id} b={b} affected={affectedSlots.includes(b.slot)}/>)}
    <g className="city-landscape" style={timed(3.05)}><Tree x={115} y={334} size={.8}/><Tree x={144} y={350} size={.7}/><Tree x={296} y={420} size={.85}/><Tree x={496} y={512} size={.9}/><Tree x={820} y={516} size={.8}/><Tree x={842} y={504} size={.7}/><Tree x={665} y={601} size={.85}/><Tree x={692} y={586} size={.7}/></g>
    <g className="city-landscape" style={timed(3.3)} fill="#f2e7b9" stroke="#85947b" strokeWidth="1"><path d="M502 291v-15m-3 0h6M887 466v-15m-3 0h6M302 370v-15m-3 0h6M573 536v-15m-3 0h6"/></g>
  </g>;
}
