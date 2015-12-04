FLY_ACIDIR := $(BUILDDIR)/aci-for-fly-flavor
FLY_ACIROOTFSDIR := $(FLY_ACIDIR)/rootfs
FLY_TOOLSDIR := $(TOOLSDIR)/fly
FLY_STAMPS :=
FLY_SUBDIRS := run gc enter aci
FLY_STAGE1 := $(BINDIR)/stage1-fly.aci

$(call setup-stamp-file,FLY_STAMP,aci-build)

$(call inc-many,$(foreach sd,$(FLY_SUBDIRS),$(sd)/$(sd).mk))

$(call generate-stamp-rule,$(FLY_STAMP),$(FLY_STAMPS) $(ACTOOL_STAMP),, \
	$(call vb,vt,ACTOOL,$(call vsp,$(FLY_STAGE1))) \
	"$(ACTOOL)" build --overwrite --owner-root "$(FLY_ACIDIR)" "$(FLY_STAGE1)")

INSTALL_DIRS += \
	$(FLY_TOOLSDIR):- \
	$(FLY_ACIDIR):- \
	$(FLY_ACIROOTFSDIR):-

FLY_FLAVORS := $(call commas-to-spaces,$(RKT_STAGE1_FLAVORS))

ifneq ($(filter fly,$(FLY_FLAVORS)),)

# actually build the fly stage1 only if requested

TOPLEVEL_STAMPS += $(FLY_STAMP)

endif
