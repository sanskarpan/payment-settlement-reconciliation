#!/usr/bin/env python3
"""Read-only design evidence, not the Go application or its acceptance oracle.
Run from any directory with Python 3.10+. Does not modify supplied inputs.
"""
import csv, hashlib, json, re
from collections import Counter, defaultdict
from datetime import datetime, timezone
from decimal import Decimal
from pathlib import Path
ROOT = Path(__file__).resolve().parents[1]
DATA = ROOT / 'workingData'
D = lambda s: Decimal(s.replace(',', '') or '0')
N = lambda s: re.sub(r'\s+', '_', s.strip()).upper()

def date(s):
    s = re.sub(r'GMT([+-])(\d{1,2})$', lambda m: 'GMT'+m[1]+m[2].zfill(2)+'00', s)
    return datetime.strptime(s, '%d %B %Y %I:%M:%S %p GMT%z').astimezone(timezone.utc).date().isoformat()

def load():
    with (DATA/'amazon_payments_data.csv').open(newline='', encoding='utf-8-sig') as f:
        rows=list(csv.reader(f))
    p=[dict(zip(rows[9],r), source_line=i) for i,r in enumerate(rows[10:],11)]
    with (DATA/'amazon_settlements_data.txt').open(newline='', encoding='utf-8-sig') as f:
        s=[dict(r,source_line=i) for i,r in enumerate(csv.DictReader(f,delimiter='\t'),2)]
    configs={}
    for side,name in [('payment','amazon_payment_configs_au_old.csv'),('settlement','amazon_settlement_configs_au.csv')]:
        with (DATA/name).open(newline='', encoding='utf-8-sig') as f:
            configs[side]=[dict(r,source_line=i) for i,r in enumerate(csv.DictReader(f),2)]
    return p,s,configs

def key(side,r,template,mode='release'):
    if side=='payment':
        rawdate=r['Transaction Release Date'] if mode=='release' and r['Transaction Release Date'] else r['date/time']
        values={'txn_ref':r['order ID'],'sku':r['sku'],'date':date(rawdate),'settlement_id':r['settlement ID'],'description':N(r['description'])}
    else:
        values={'txn_ref':r['order-id'] or r['adjustment-id'],'sku':r['sku'],'date':datetime.strptime(r['posted-date'],'%d.%m.%Y').date().isoformat(),'settlement_id':r['settlement-id'],'shipment_id':r['shipment-id'],'merchant_order_id':r['merchant-order-id'],'description':N(r['amount-description'])}
    return tuple(values.get(t,t) for t in template.split('+'))

def analyze(p,s,configs,fixed):
    # Explicit diagnostic semantics: all equal-specificity routes retained and disclosed.
    cfg={k:[dict(r) for r in v] for k,v in configs.items()}
    if fixed:
        cfg['payment']=[r for r in cfg['payment'] if r['source_line'] not in (5,72)]
        for r in cfg['payment']:
            if r['source_line'] in (14,):
                r['to_summary_field_when_positive_amount']=r['to_summary_field_when_negative_amount']='refunded_expenses'
        for r in cfg['settlement']:
            if r['source_line'] in (61,63,95,118):
                r['to_summary_field_when_positive_amount']=r['to_summary_field_when_negative_amount']='sales_product_charges'
    buckets={side:defaultdict(Decimal) for side in cfg}
    groups={side:defaultdict(lambda: {'count':0,'amount':Decimal(0),'lines':[],'buckets':defaultdict(Decimal)}) for side in cfg}
    routes=[];issues=[]
    for side,rows in [('payment',p),('settlement',s)]:
        bytx=defaultdict(list)
        for c in cfg[side]:bytx[N(c['transaction_type'])].append(c)
        for row in rows:
            if side=='payment' and (row['settlement ID']!='12395580393' or row['Transaction status']!='Released'):continue
            if side=='settlement' and not row['transaction-type']:continue
            tx=N(row['type' if side=='payment' else 'transaction-type'])
            desc=N(row['description' if side=='payment' else 'amount-description'])
            # Documented source-label alias, not an amount or bucket correction.
            if side=='payment' and tx=='TRANSFER' and desc.startswith('TO_ACCOUNT_ENDING_WITH:'):
                desc='TO_ACCOUNT_ENDING'
            def matches(c):
                cd=c['description' if side=='payment' else 'amount_description']
                return (cd=='any' or N(cd)==desc) and (side=='payment' or N(c['amount_type'])==N(row['amount-type']))
            candidates=[c for c in bytx[tx] if matches(c)]
            if not candidates:candidates=[c for c in bytx[''] if matches(c)]
            selected=[]
            for field in sorted(set(c.get('amount_field','amount') for c in candidates)):
                cc=[c for c in candidates if c.get('amount_field','amount')==field]
                exact=[c for c in cc if c['description' if side=='payment' else 'amount_description']!='any']
                selected.extend(exact or cc)
            if not selected:issues.append({'side':side,'line':row['source_line'],'type':'unmapped'});continue
            keys={key(side,row,c['record_ref']) for c in selected if c['record_ref']}
            if len(keys)!=1:issues.append({'side':side,'line':row['source_line'],'type':'multiple_or_empty_key'});continue
            k=next(iter(keys));g=groups[side][k]
            g['count']+=1;g['amount']+=D(row['total' if side=='payment' else 'amount']);g['lines'].append(row['source_line'])
            for c in selected:
                field=c.get('amount_field','amount')
                column={'fba_fees':'fulfilment by amazon fees'}.get(field,field.replace('_',' ')) if side=='payment' else 'amount'
                amount=D(row.get(column,'0'))
                b=c['to_summary_field_when_positive_amount' if amount>=0 else 'to_summary_field_when_negative_amount']
                if amount and b:
                    buckets[side][b]+=amount;g['buckets'][b]+=amount
                    routes.append({'source':side,'source_line':row['source_line'],'config_line':c['source_line'],'field':field,'bucket':b,'amount':str(amount),'record_ref_parts':k})
    pg,sg=groups['payment'],groups['settlement'];both=pg.keys()&sg.keys()
    variances=[]
    for k in sorted(both):
        for b in sorted(pg[k]['buckets'].keys()|sg[k]['buckets'].keys()):
            delta=pg[k]['buckets'][b]-sg[k]['buckets'][b]
            if delta:variances.append({'key':k,'bucket':b,'difference':str(delta),'payment_lines':pg[k]['lines'],'settlement_lines':sg[k]['lines']})
    return {'summary':{b:{'payment':str(buckets['payment'][b]),'settlement':str(buckets['settlement'][b]),'difference':str(buckets['payment'][b]-buckets['settlement'][b])} for b in sorted(buckets['payment'].keys()|buckets['settlement'].keys())},'keys':{'payment':len(pg),'settlement':len(sg),'both':len(both),'payment_only':len(pg.keys()-sg.keys()),'settlement_only':len(sg.keys()-pg.keys()),'matched_amount_differences':sum(pg[k]['amount']!=sg[k]['amount'] for k in both)},'payment_only':[{'key':k,**pg[k]} for k in sorted(pg.keys()-sg.keys())],'issues':issues,'group_bucket_variances_sample':variances[:20],'config_diagnostics':config_diagnostics(cfg),'rule_controls':rule_controls(routes),'group_bucket_variance_count':len(variances)}

def config_diagnostics(configs):
    diagnostics=[]
    for side,rules in configs.items():
        selectors=defaultdict(list)
        for r in rules:
            cols=['transaction_type','description','amount_field'] if side=='payment' else ['transaction_type','amount_type','amount_description']
            selectors[tuple(N(r[c]) for c in cols)].append(r['source_line'])
        for selector,lines in selectors.items():
            if len(lines)>1:diagnostics.append({'code':'DUPLICATE_SELECTOR','source':side,'selector':selector,'config_lines':lines})
    return diagnostics

def rule_controls(routes):
    controls={}
    for r in routes:
        k=(r['source'],r['config_line'],r['field'],r['bucket'])
        if k not in controls: controls[k]={'source':k[0],'config_line':k[1],'field':k[2],'bucket':k[3],'count':0,'amount':Decimal(0),'source_lines':[]}
        c=controls[k];c['count']+=1;c['amount']+=Decimal(r['amount']);c['source_lines'].append(r['source_line'])
    return list(controls.values())

def main():
    p,s,c=load()
    profile={'files':{x.name:{'bytes':x.stat().st_size,'sha256':hashlib.sha256(x.read_bytes()).hexdigest()} for x in sorted(DATA.iterdir()) if x.is_file()},'payment_rows':len(p),'settlement_rows_including_metadata':len(s),'config_rows':{k:len(v) for k,v in c.items()},'payment_scope':[]}
    for sid,status in sorted(set((r['settlement ID'],r['Transaction status']) for r in p)):
        rows=[r for r in p if (r['settlement ID'],r['Transaction status'])==(sid,status)]
        profile['payment_scope'].append({'settlement_id':sid,'status':status,'count':len(rows),'total':str(sum(D(r['total']) for r in rows))})
    profile['settlement_component_total']=str(sum(D(r['amount']) for r in s))
    out=ROOT/'docs/evidence';out.mkdir(parents=True,exist_ok=True)
    for name,value in [('input-profile',profile),('mapping-before-probe',analyze(p,s,c,False)),('mapping-after-probe',analyze(p,s,c,True))]:
        (out/(name+'.json')).write_text(json.dumps(value,indent=2,default=str)+'\n')
        print(name, {k:v for k,v in value.items() if k in ['summary','keys','issues','payment_rows','settlement_component_total']})
if __name__=='__main__':main()
