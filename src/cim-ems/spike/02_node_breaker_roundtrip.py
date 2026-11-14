import pypowsybl.network as pn, pypowsybl.loadflow as lf, os, sys, zipfile, time
out=sys.argv[1]
n = pn.create_four_substations_node_breaker_network()
print("switches", len(n.get_switches()), "ptc", len(n.get_phase_tap_changers()), "rtc", len(n.get_ratio_tap_changers()))
t=time.time(); r=lf.run_ac(n); print("LF", r[0].status, round(time.time()-t,3),"s")
sw=n.get_switches(); b=sw[(~sw.open)&(sw.kind=="BREAKER")].index[0]
print("buses before", len(n.get_bus_breaker_view_buses() if hasattr(n,'get_bus_breaker_view_buses') else n.get_buses()))
n.update_switches(id=b, open=True)
r=lf.run_ac(n); print("open",b,r[0].status, "bus-view buses", len(n.get_buses()))
n.update_switches(id=b, open=False)
# tap change
rt=n.get_ratio_tap_changers()
if len(rt): 
    k=rt.index[0]; n.update_ratio_tap_changers(id=k, tap=int(rt.loc[k,'tap'])); print("tap ok", k)
# SLD with metadata
vl=n.get_voltage_levels().index[0]
d=n.get_single_line_diagram(vl); open(out+"/sld.svg","w").write(d.svg)
print("SLD bytes", len(d.svg)); print("metadata type", type(d.metadata), str(d.metadata)[:400])
# export all profiles
n.save(out+"/exp_nb.zip", format="CGMES", parameters={"iidm.export.cgmes.profiles":"EQ,SSH,SV,TP","iidm.export.cgmes.cgmes-export-topology-kind":"NODE_BREAKER"} if False else {"iidm.export.cgmes.profiles":"EQ,SSH,SV,TP"})
print([ (i.filename,i.file_size) for i in zipfile.ZipFile(out+"/exp_nb.zip").infolist()])
n2=pn.load(out+"/exp_nb.zip",parameters={"iidm.import.cgmes.import-node-breaker-as-bus-breaker":"false"})
print("reimport sw", len(n2.get_switches()), "lines",len(n2.get_lines()), "2wt",len(n2.get_2_windings_transformers()), "gens", len(n2.get_generators()))
r=lf.run_ac(n2); print("LF reimp", r[0].status)
# compare flows
n.update_switches(id=b, open=False); lf.run_ac(n)
a=n.get_lines()[['p1','q1']]; c=n2.get_lines()[['p1','q1']]
print("max dP", float((a.p1-c.p1).abs().max()), "max dQ", float((a.q1-c.q1).abs().max()))
# measurements extension?
print([e for e in pn.get_extensions_names() if 'eas' in e or 'cgmes' in e.lower()])
