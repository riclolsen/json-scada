import pypowsybl as pp, pypowsybl.network as pn, pypowsybl.loadflow as lf, os, sys
print("pypowsybl", pp.__version__ if hasattr(pp,"__version__") else "?")
print([f for f in dir(pn) if "micro" in f.lower() or "cgmes" in f.lower() or "ieee" in f.lower()])
n = pn.create_micro_grid_be_network() if hasattr(pn,"create_micro_grid_be_network") else None
print(n)
print("formats", pn.get_import_formats())
print("export", pn.get_export_formats())
r = lf.run_ac(n)
print("LF", r[0].status, r[0].iteration_count)
print(n.get_buses().head(3))
print("switches", len(n.get_switches()), "2wt", len(n.get_2_windings_transformers()), "lines", len(n.get_lines()))
sw = n.get_switches()
print(sw.head(3))
# flip one closed breaker
cl = sw[~sw.open & (sw.kind=="BREAKER")].index[0]
n.update_switches(id=cl, open=True)
r2 = lf.run_ac(n); print("after open", cl, r2[0].status, len(n.get_buses()))
n.update_switches(id=cl, open=False)
# diagram
vl = n.get_voltage_levels().index[0]
svg = n.get_single_line_diagram(vl)
open(sys.argv[1]+"/sld.svg","w").write(svg.svg); print("sld bytes", len(svg.svg), "meta", len(svg.metadata))
nad = n.get_network_area_diagram()
open(sys.argv[1]+"/nad.svg","w").write(nad.svg); print("nad bytes", len(nad.svg))
# export CGMES
try:
    n.save(sys.argv[1]+"/exp.zip", format="CGMES", parameters={"iidm.export.cgmes.profiles":"EQ,SSH,SV,TP"})
    print("exported", os.listdir(sys.argv[1]))
    n2 = pn.load(sys.argv[1]+"/exp.zip"); print("reimport", len(n2.get_buses()), len(n2.get_lines()))
except Exception as e: print("export err", repr(e)[:300])
