package resolver

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/seosite/protodep/pkg/config"
	"github.com/seosite/protodep/pkg/logger"
)

func (s *resolver) ResolveLocal(forceUpdate bool, cleanupCache bool) error {

	dep := config.NewDependency(s.conf.TargetDir, forceUpdate)
	protodep, err := dep.Load()
	if err != nil {
		return err
	}

	newdeps := make([]config.ProtoDepDependency, 0, len(protodep.Dependencies))
	protodepDir := filepath.Join(s.conf.HomeDir, ".protodep")

	_, err = os.Stat(protodepDir)
	if cleanupCache && err == nil {
		files, err := os.ReadDir(protodepDir)
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.IsDir() {
				dirpath := filepath.Join(protodepDir, file.Name())
				if err := os.RemoveAll(dirpath); err != nil {
					return err
				}
			}
		}
	}

	outdir := filepath.Join(s.conf.OutputDir, protodep.ProtoOutdir)
	// 不删除文件
	// if err := os.RemoveAll(outdir); err != nil {
	// 	return err
	// }

	for _, dep := range protodep.Dependencies {
		sources := make([]protoResource, 0)

		compiledIgnores := compileIgnoreToGlob(dep.Ignores)
		compiledIncludes := compileIgnoreToGlob(dep.Includes)

		hasIncludes := len(dep.Includes) > 0

		protoRootDir := dep.Target
		filepath.Walk(protoRootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, ".proto") || strings.HasSuffix(path, ".pb.go") {
				isIncludePath := s.isMatchPath(protoRootDir, path, dep.Includes, compiledIncludes)
				isIgnorePath := s.isMatchPath(protoRootDir, path, dep.Ignores, compiledIgnores)

				if hasIncludes && !isIncludePath {
					logger.Info("skipped %s due to include setting", path)
				} else if isIgnorePath {
					logger.Info("skipped %s due to ignore setting", path)
				} else {
					sources = append(sources, protoResource{
						source:       path,
						relativeDest: strings.Replace(path, protoRootDir, "", -1),
					})
				}
			}
			return nil
		})

		for _, s := range sources {
			outpath := filepath.Join(outdir, dep.Path, s.relativeDest)

			content, err := os.ReadFile(s.source)
			if err != nil {
				return err
			}

			if err := writeFileWithDirectory(outpath, content, 0644); err != nil {
				return err
			}
		}

		newdeps = append(newdeps, config.ProtoDepDependency{
			Target: dep.Target,
		})
	}

	newProtodep := config.ProtoDep{
		ProtoOutdir:  protodep.ProtoOutdir,
		Dependencies: newdeps,
	}

	if dep.IsNeedWriteLockFile() {
		if err := writeToml("protodep.lock", newProtodep); err != nil {
			return err
		}
	}

	return nil
}
