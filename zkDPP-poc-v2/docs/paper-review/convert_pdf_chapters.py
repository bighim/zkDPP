"""Reconstruct editable chapter TeX from the supplied two-column manuscript.

No protocol rewrite: equations and tables are transcribed, prose follows PDF
column order. Keep source page comments and a source PDF for checking typography.
"""
from pathlib import Path
import re, json, shutil, hashlib, sys
import pdfplumber

HERE = Path(__file__).resolve().parent
SOURCE = Path('/Users/bighim/Downloads/zkdpp-v0.2-260916.pdf')
OUT = HERE / 'zkdpp-v0.2-260916'
NAMES = ['00.Abstract', '01.Introduction', '02.Related-Work', '03.System-and-Trust-Model',
         '04.Objects-and-Ledger-State', '05.Transitions-and-Accounting',
         '06.Public-DPP-Verification', '07.Tracing-and-Freezing',
         '08.Security-Requirements', '09.Security-Proofs',
         '10.Model-Validation', '11.Scope-and-Limitations', '12.Conclusion', '13.References']
ROMAN = dict(zip(['I','II','III','IV','V','VI','VII','VIII','IX','X','XI','XII'], range(1,13)))
EQ = {
1: r'W_B=\operatorname{TE.Enc}(PK_A,\operatorname{EncodeSecret}(SK_B),\eta_A;\mathrm{ctx}_{\mathrm{wrap}}).',
2: r'm\leftarrow\operatorname{AuditDec}(sk_{\mathrm{audit}},ct;\mathrm{ctx})',
3: r'\begin{gathered}y=\left\lfloor\frac{xq}{Q}\right\rfloor,\quad Q>0,\\xq=Qy+r,\quad 0\le r<Q.\end{gathered}',
4: r'\begin{gathered}n=(d,\rho,q,a,e,o,\omega_N),\\cm=H_{\mathrm{Note}}(d,\rho,q,a,e,o,\omega_N)\end{gathered}',
5: r'nf=H_{\mathrm{Nullifier}}(cm,sk)',
6: r'\begin{gathered}v=(d,\rho,q,a,e,o_S,o_R,D,\omega_V),\\rv=H_{\mathrm{Voucher}}(d,\rho,q,a,e,o_S,o_R,D,\omega_V).\end{gathered}',
7: r'\begin{gathered}s_{\mathrm{res}}=H_{\mathrm{VoucherResolutionSecret}}(\omega_V),\\rvnf=H_{\mathrm{VoucherNullifier}}(rv,s_{\mathrm{res}})\end{gathered}',
8: r'\mathrm{DocumentInfo}=(\mathrm{ProductName},\mathrm{LotID},\mathrm{Unit})',
9: r'h=H_{\mathrm{Issue}}(d,p,c),\quad\mathrm{DPPClaim}=(p,h,c)',
10: r'\begin{gathered}\mathrm{Active}\longrightarrow\mathrm{Frozen},\\\mathrm{Frozen}\longrightarrow\{\mathrm{Active},\mathrm{Revoked}\}\end{gathered}',
11: r'\begin{aligned}r_i^{\mathrm{out}}&=\operatorname{Ref}(z_i^{\mathrm{out}}),\\\nu_i^{\mathrm{in}}&=\operatorname{SpendId}(z_i^{\mathrm{in}},sk_i),\\ct_j&=\operatorname{AuditEnc}(pk_{\mathrm{audit}},M_j(w);\eta_j,\mathrm{ctx}_j)\end{aligned}',
12: r'R_k=R_k^{\mathrm{core}}\land R_k^{\mathrm{acct}}',
13: r'\begin{aligned}q_{\mathrm{in}}&=q_T+q_C,\quad q_T>0,\quad q_C\ge0,\\a_C&=\left\lfloor\frac{a_{\mathrm{in}}q_C}{q_{\mathrm{in}}}\right\rfloor,\quad a_T=a_{\mathrm{in}}-a_C,\\e_C&=\left\lfloor\frac{e_{\mathrm{in}}q_C}{q_{\mathrm{in}}}\right\rfloor,\\e_T&=e_{\mathrm{in}}-e_C+\delta_T\end{aligned}',
14: r'\operatorname{CurrentEpoch}()\le D',
15: r'(q,a,e)_{\mathrm{out}}=(q_1+q_2,a_1+a_2,e_1+e_2)',
16: r'\begin{aligned}a_2&=\left\lfloor\frac{a_{\mathrm{in}}q_2}{q_{\mathrm{in}}}\right\rfloor,\quad a_1=a_{\mathrm{in}}-a_2,\\e_2&=\left\lfloor\frac{e_{\mathrm{in}}q_2}{q_{\mathrm{in}}}\right\rfloor,\quad e_1=e_{\mathrm{in}}-e_2\end{aligned}',
17: r'\begin{gathered}Q_{\mathrm{in}}=Q_E+Q_W+q_{\mathrm{loss}},\\q_{\mathrm{loss}}\ge0,\quad Q_E>0\end{gathered}',
18: r'\sum_i a_i^{\mathrm{in}}=\sum_{j\in I_E}a_j^{\mathrm{out}}',
19: r'\begin{aligned}E&=\sum_i e_i^{\mathrm{in}}+\delta_P,\quad\delta_P\ge0,\\e_j^{\mathrm{out}}&=\left\lfloor\frac{E q_j^{\mathrm{out}}}{Q_E}\right\rfloor\quad(j\in I_E,\ j\ne j_0),\\e_{j_0}^{\mathrm{out}}&=E-\sum_{j\in I_E\setminus\{j_0\}}e_j^{\mathrm{out}}\end{aligned}',
20: r'v_Ra\ge u_Rq,\quad v_Ce\le u_Cq',
21: r'T=F_N\cup F_V,\quad t=(r,\nu)',
22: r'\begin{aligned}\operatorname{Blocked}_{H_r}(t)\iff{}&\operatorname{spentIn}_{H_r}^{\kappa_t}[\nu_t]=0\\&\land\operatorname{status}_{H_r}^{\kappa_t}[\nu_t]\in\{\mathrm{Frozen},\mathrm{Revoked}\}.\end{aligned}',
23: r'\Pr[\mathrm{Fail}_{\mathrm{corr}}]\le Q\epsilon_{\mathrm{comp}}+\epsilon_E+\epsilon_H.',
24: r'\Pr[\mathrm{Exp}_{\mathrm{sound}}=1]\le\epsilon_B:=\epsilon_X+\epsilon_C+\epsilon_M+\epsilon_H+\epsilon_E.',
25: r'T\ne\varnothing,\quad\forall t\in T:\operatorname{Blocked}_{H_r}(t).',
26: r'M[k(r)]=r,',
27: r'\mathrm{Adv}_{\mathrm{hist}}\le2(\epsilon_Z+\epsilon_A+\epsilon_D+\epsilon_{\mathrm{RO}}).',
}
# Page, column (0 left), top, bottom, equation number; coordinates in PDF points.
REGIONS=[(5,0,186,230,1),(5,0,442,462,2),(5,1,491,557,3),(5,1,728,768,4),
 (6,0,84,107,5),(6,0,170,211,6),(6,0,259,299,7),(6,0,400,422,8),(6,0,485,509,9),(6,1,172,213,10),
 (7,0,194,253,11),(7,0,684,713,12),(8,0,338,471,13),(8,0,698,721,14),
 (8,1,112,135,15),(8,1,197,265,16),(8,1,444,481,17),(8,1,488,521,18),(8,1,555,665,19),
 (9,0,181,204,20),(10,0,581,607,21),(10,1,593,641,22),(12,0,344,372,23),
 (12,1,776,798,24),(13,1,346,372,25),(14,0,631,655,26),(15,0,91,117,27)]
FLOATS=[(4,-1,45,353),(7,0,45,179),(8,0,45,282),(10,1,45,178),(16,-1,45,194),(16,0,198,478)]

def esc(s):
    s=s.replace('ﬁ','fi').replace('ﬀ','ff').replace('ﬂ','fl').replace('\u00ad','').replace(';',';')
    return ''.join({'\\':r'\textbackslash{}','&':r'\&','%':r'\%','$':r'\$','#':r'\#','_':r'\_','{':r'\{','}':r'\}','~':r'\textasciitilde{}','^':r'\textasciicircum{}'}.get(c,c) for c in s)

def mathchar(s):
    return s.replace('{',r'\{').replace('}',r'\}').replace('⃗',r'\vec{\vphantom{x}}').replace('̸',r'\not ').replace('&',r'\&').replace('#',r'\#').replace('%',r'\%').replace('∅',r'\varnothing ').replace('ϵ',r'\epsilon ').replace('ℓ',r'\ell ').replace('⋆',r'\star ')

def render_line(line):
    chars=sorted(line['chars'],key=lambda c:c['x0'])
    out=[];buf=[];mode=False;last=None
    def flush():
        nonlocal buf
        if not buf:return
        if mode:
            main=[c for c in buf if c['size']>8]
            top=min((c['top'] for c in main),default=buf[0]['top'])
            result='';j=0
            while j<len(buf):
                c=buf[j]
                if c['size']<8 and main:
                    sub=c['top']>top+1;part=''
                    while j<len(buf) and buf[j]['size']<8 and (buf[j]['top']>top+1)==sub:
                        part+=buf[j]['text'];j+=1
                    result+=('_{' if sub else '^{')+mathchar(part)+'}'
                else: result+=mathchar(c['text']);j+=1
            out.append(r'\('+result+r'\)')
        else:out.append(esc(''.join(c['text'] for c in buf)))
        buf=[]
    for c in chars:
        font=c['fontname'].split('+')[-1]
        m=font.startswith(('CM','MSAM','MSBM')) or (c['size']<8 and mode)
        if last and c['x0']-last['x1']>1.15 and not (m and mode):
            flush();out.append(' ')
        if m!=mode:flush();mode=m
        buf.append(c);last=c
    flush()
    return ''.join(out)

def table(num,caption,heads,rows):
    cols='@{}'+('X'*len(heads))+'@{}'
    return '\n'+r'\begin{table}[htbp]\centering\small'+'\n'+f'\\setcounter{{table}}{{{num-1}}}\\caption{{{caption}}}\n'+r'\begin{tabularx}{\linewidth}{'+cols+'}\\toprule\n'+ ' & '.join(r'\textbf{'+h+'}' for h in heads)+r' \\ \midrule'+'\n'+('\n'.join(' & '.join(row)+r' \\' for row in rows))+'\n'+r'\bottomrule\end{tabularx}\end{table}'+'\n'

TABLES={
2:table(1,'Closest mechanisms and the scope of comparison',['Baseline','Related mechanism','Distinction for comparison with zkDPP'],[
['Udokwu--Craß [3]','Private-input sustainability predicates','Accepted lifecycle graph and live-object response'],
['Babel et al. [2]','Linked credentials, private ownership transfer, fraction consumption','Post hoc targets and current consumption checks'],
['Sahai et al. [4]','Supply-chain consume/create rules and public input document references','Hidden actual parent correspondence and authorized recovery'],
['GGM [7]','Proof-bound tracing data and expansion to descendant outputs','Typed lifecycle, enforcement targets and completion'],
['Azeroth [8]','Transaction-bound encrypted audit information','Explicit graph traversal and whole-target response'],
['Lee et al. [9]','Flow-, direction- and epoch-scoped audit capability','Key-domain-wide auditor access; separate containment procedure'],
['CAP [5]','Policy-controlled viewing and freezing; consume-and-reissue freeze','Full-reveal/threshold conditions; shared spend ID versus encrypted future ID'],
['ERC-20R [11]','Downstream amount tracing and atomic freezing on public history','Account liability versus full descendant sets; atomic execution versus spending races'],
['ACTMeds [12]','Private recall-reason proofs; transitions restricted by batch status','Batch-level restrictions versus all recovered live descendants'],
['ZEXE family and composed baseline [14, 15]','Generic private creation and consumption rules','Same trust, key access and leakage; same application relations']]),
5:table(2,'Verification and authorization routes',['Events','Verifier','Policy and scope'],[['Ordinary seven','Fixed per event','No registry or scope grant'],['Process','Registered policy','Policy and scope grant'],['Issue','Registered policy','Policy only']])+table(3,'Ordered event inputs and outputs',['Event','Transition','Parents','Future IDs'],[
['Entry',r'$\varnothing\to N$','0','1'],['Transfer',r'$N\to(V,N_C)$','1','2'],['Proceed',r'$V\to N_R$','1','1'],['Recall',r'$V\to N_S$','1','1'],['Merge',r'$(N_1,N_2)\to N$','2','1'],['Split',r'$N\to(N_1,N_2)$','1','2'],['Process',r'$N^m\to N^n$','$m$','$n$'],['Exit',r'$N\to\varnothing$','1','0'],['Issue',r'$N\to C$','1','0']]),
7:table(4,'Per-target action at execution time',['Observed state','Action in this run'],[['Unspent, Active','Submit a typed Freeze transaction'],['Unspent, Frozen/Revoked','Record an existing block'],['Already spent','Retain target and consumer as a race'],['Read failure','Retain target as unconfirmed']]),
10:table(5,'Symbolic checks and their observed outcomes',['Case','Outcome in the finite model'],[['All fixture starts and prefixes','334 exact graph/frontier matches'],['Join, backward sibling, other parent','Reachable edges retained; unrelated branches excluded'],['Voucher resolution order','First consumption excludes the competing path'],['Frozen/revoked input','All eight consuming event kinds rejected'],['Missing/mistyped audit data','Trace failure'],['Partial, omitted or pending result','Incomplete response'],['Race or incompatible checkpoint','Incomplete response'],['Zero-mass carbon residual','Permitted accounting counterexample'],['Owner-secret alias','Identifier-invariance countermodel']])}

def main():
    OUT.mkdir(exist_ok='--refresh-generated' in sys.argv)
    shutil.copyfile(SOURCE,OUT/'source.pdf')
    chunks={i:[] for i in range(14)};chapter=0;seen=[];headers=[]
    with pdfplumber.open(SOURCE) as pdf:
        for no,page in enumerate(pdf.pages,1):
            for side,(x0,x1) in enumerate([(47,291),(307,549)]):
                rawlines=page.crop((x0,48,x1,795)).extract_text_lines(x_tolerance=1,y_tolerance=3)
                lines=[l for l in rawlines if max(c['size'] for c in l['chars'])>=8]
                for small in [l for l in rawlines if max(c['size'] for c in l['chars'])<8]:
                    near=min(lines,key=lambda l:abs(l['top']-small['top']))
                    if abs(near['top']-small['top'])<8:
                        near['chars'].extend(small['chars'])
                    else:lines.append(small)
                entries=[]
                for line in lines:
                    y=line['top']
                    if no==1 and y<210:continue
                    if any(p==no and (c==side or c==-1) and lo<=y<hi for p,c,lo,hi in FLOATS):continue
                    if any(p==no and c==side and lo<=y<hi for p,c,lo,hi,n in REGIONS):continue
                    entries.append((y,'line',line))
                for p,c,lo,hi,n in REGIONS:
                    if p==no and c==side:entries.append((lo,'eq',n))
                prev_bottom=None
                chunks[chapter].append(f'\n% Source PDF page {no}, column {side+1}\n')
                for y,kind,value in sorted(entries,key=lambda x:x[0]):
                    if kind=='eq':
                        chunks[chapter].append(f'\n\\begin{{equation}}\n{EQ[value]}\\tag{{{value}}}\n\\end{{equation}}\n');seen.append(value);prev_bottom=None;continue
                    line=value;t=line['text'];large=max(c['size'] for c in line['chars'])
                    h=re.match(r'^([IVX]+)\s+(.+)$',t)
                    sub=re.match(r'^(\d+)\.(\d+)\s+(.+)$',t)
                    if h and large>11:
                        chapter=ROMAN[h[1]];chunks[chapter].append(f'\\section{{{esc(h[2])}}}\n');headers.append((chapter,no,t));prev_bottom=None;continue
                    if t=='References':chapter=13;chunks[chapter].append(r'\section*{References}'+'\n');prev_bottom=None;continue
                    if sub and large>11:
                        chunks[chapter].append(f'\n\\subsection{{{esc(sub[3])}}}\n');prev_bottom=None;continue
                    if chapter<=1:continue # use manually verified Abstract and Introduction below
                    if chapter!=13 and prev_bottom is not None and (y-prev_bottom>5 or line['x0']>x0+6):chunks[chapter].append('\n')
                    if chapter==13 and re.match(r'^\[\d+\]',t):chunks[chapter].append('\n')
                    chunks[chapter].append(render_line(line)+'\n');prev_bottom=line['bottom']
    if sorted(seen)!=list(range(1,28)):raise RuntimeError(('equation coverage',seen))
    original=(HERE/'zkdpp-v0.2-260916-introduction.tex').read_text()
    abstract=original[original.index('\\section*{Abstract'):original.index('\\section{서론}')]
    intro=original[original.index('\\section{서론}'):original.index('\\section*{서론에서 인용한')]
    chunks[0]=[abstract.replace('\\clearpage','')]
    chunks[1]=[intro.replace('\\clearpage','')]
    for i in range(14):
        text=''.join(chunks[i])
        if i in TABLES:text+='\n'+TABLES[i]
        if i==10:
            text+=r'''\begin{figure}[htbp]\centering
\includegraphics[page=16,trim=46 687 44 47,clip,width=\linewidth]{source.pdf}
\caption{Worked accounting flow after accepted production and transfer. The dashed edge is an actual input edge outside the forward traversal from A; the figure shows it only to explain the process accounting.}
\end{figure}
'''
        (OUT/(NAMES[i]+'.tex')).write_text(text)
    pre=r'''\documentclass[11pt,a4paper]{article}
\usepackage{kotex}
\usepackage[margin=22mm]{geometry}
\usepackage{amsmath,amssymb,unicode-math,graphicx,booktabs,tabularx}
\setmathfont{latinmodern-math.otf}
\usepackage[hidelinks]{hyperref}
\setlength{\parindent}{0pt}
\setlength{\parskip}{6pt}
\setlength{\emergencystretch}{3em}
\renewcommand{\arraystretch}{1.18}
\begin{document}
\begin{center}{\LARGE\bfseries zkDPP: 비공개 공급망 회계와\\승인된 이력 추적 및 사용 차단}\par\medskip
zkDPP: Private supply-chain accounting,\\authorized provenance tracing, and use prevention
\end{center}
'''
    body='\n'.join(('\\clearpage\n' if i else '')+f'\\input{{{name}.tex}}' for i,name in enumerate(NAMES))
    (OUT/'main.tex').write_text(pre+body+'\n\\end{document}\n')
    (OUT/'conversion-manifest.json').write_text(json.dumps({'source':str(SOURCE),'sha256':hashlib.sha256(SOURCE.read_bytes()).hexdigest(),'sourcePages':18,'chapters':NAMES,'equations':sorted(seen),'tables':5,'figure':'original vector crop from source.pdf page 16','headings':headers,'status':'PDF transcription; not original author TeX; review inline math and line-wrap normalization against source'},ensure_ascii=False,indent=2)+'\n')
    print(OUT)
if __name__=='__main__':main()
